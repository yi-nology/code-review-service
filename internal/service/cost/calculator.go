// Package cost 实现 LLM 成本估算：根据 review 的 token 用量和模型单价计算估算成本，
// 并提供按 provider/项目/模型维度的成本统计与预算检查。
//
// 成本估算公式：input_tokens × input_price + output_tokens × output_price（每百万 token 单价）。
// 单价来自 llm_model_pricing 表（管理员在 Web 控制台维护）；无单价的模型按 0
// 计算（不阻断，仅缺数据）。
package cost

import (
	"context"
	"strings"
	"time"

	"github.com/yi-nology/code-review-service/internal/domain/pricing"
	"github.com/yi-nology/code-review-service/internal/repository"

	"github.com/oekit/logx"
	"github.com/oekit/pkg/cache"
)

// 单价缓存策略。
const (
	// pricingCacheTTL 单价条目的缓存时长。管理员改了单价后，最多过这么久
	// 才会生效——热路径每次评审都要读单价，不缓存等于每次都查库。
	pricingCacheTTL = 5 * time.Minute
	// pricingNullTTL 无单价模型的空值缓存时长。短：模型单价随时可能被补上，
	// 但也不能太长——空值放太久，新建的模型会一直按 0 计价。
	pricingNullTTL = time.Minute
	// pricingCacheMaxSize 缓存条目上限。模型数量有限，留足余量即可。
	pricingCacheMaxSize = 256
)

// Calculator 计算 LLM 调用的估算成本。
//
// 单价走 Cache-Aside：命中内存缓存直接用，未命中回源仓储并按 TTL 缓存。
type Calculator struct {
	repo  repository.PricingRepository
	cache *cache.LocalCache[ModelPrice]
}

// ModelPrice 是单个模型的单价（每百万 token 美元）。
type ModelPrice struct {
	Input      float64
	Output     float64
	CacheRead  float64
	CacheWrite float64
}

// NewCalculator 用单价仓储构造成本估算器。
func NewCalculator(repo repository.PricingRepository) *Calculator {
	c := &Calculator{repo: repo}
	c.cache = cache.NewLocalCache(cache.LocalConfig[ModelPrice]{
		TTL:     pricingCacheTTL,
		NullTTL: pricingNullTTL,
		MaxSize: pricingCacheMaxSize,
		// 回源失败时回退已过期的历史单价（fail-open）：DB 抖一下不该让
		// 成本估算变成 0——那会让预算检查以为「很便宜」。
		StaleIfError: true,
		Source:       c.loadPrice,
	})
	return c
}

// Estimate 按 token 用量和模型单价估算单次调用成本（美元）。
// provider/model 用于查单价；若无匹配单价返回 0（不阻断，仅缺数据）。
func (c *Calculator) Estimate(ctx context.Context, provider, model string, inputTokens, outputTokens, cacheRead, cacheWrite int64) float64 {
	if c == nil {
		return 0
	}
	price := c.lookup(ctx, provider, model)
	// 单价是「每百万 token」，token 数 / 1e6 × 单价
	const mega = 1_000_000.0
	return float64(inputTokens)/mega*price.Input +
		float64(outputTokens)/mega*price.Output +
		float64(cacheRead)/mega*price.CacheRead +
		float64(cacheWrite)/mega*price.CacheWrite
}

// lookup 查询单价（带 TTL 缓存）。
//
// provider 已知时按 (provider, model) 精确匹配；provider 未知时按 model 名
// 兜底——月度用量只按 model 聚合，拿不到 provider（见
// repository/postgres/review.go 的 MonthlyTokenUsage）。
//
// 此前这里只有 (provider, model) 一种键，且 provider 为空时拼出的键形如
// "|gpt-4o" 永远匹配不上表里的 "openai|gpt-4o"——月度预算检查因此恒为 0，
// 从不触发（block 模式也从不拦）。兜底的存在不是锦上添花，是修这个洞。
func (c *Calculator) lookup(ctx context.Context, provider, model string) ModelPrice {
	if c == nil || c.repo == nil {
		return ModelPrice{}
	}
	key := provider + "|" + model
	price, err := c.cache.GetOrLoad(ctx, key)
	if err != nil {
		logx.CtxWarnf(ctx, "[cost] load pricing failed: provider=%s model=%s err=%v", provider, model, err)
		return ModelPrice{}
	}
	if price == nil {
		// 无该 (provider, model) 的单价：provider 未知时按 model 名再兜底一次。
		if provider == "" {
			return c.lookupByModel(ctx, model)
		}
		return ModelPrice{}
	}
	return *price
}

// lookupByModel 按模型名兜底查价。
//
// 同一模型名可能出现在多个 provider 下（单价表的唯一键是 (provider, model)）。
// 取候选里**最贵**的一条：预算是护栏，宁可高估也不让它因为取到了便宜的那条
// 而放过超支。候选为空时返回零值（无单价，按 0 计）。
func (c *Calculator) lookupByModel(ctx context.Context, model string) ModelPrice {
	candidates, err := c.repo.ListPricingByModel(ctx, model)
	if err != nil {
		logx.CtxWarnf(ctx, "[cost] list pricing by model failed: model=%s err=%v", model, err)
		return ModelPrice{}
	}
	var best ModelPrice
	found := false
	for _, p := range candidates {
		cur := ModelPrice{
			Input:      p.InputPricePerMega,
			Output:     p.OutputPricePerMega,
			CacheRead:  p.CacheReadPricePerMega,
			CacheWrite: p.CacheWritePricePerMega,
		}
		if !found || cur.moreExpensiveThan(best) {
			best = cur
			found = true
		}
	}
	return best
}

// moreExpensiveThan 比较两条单价的总档位（输入 + 输出）。
//
// 只比输入输出两档：cache 档在多数模型上为空计，取它做比较只会引入噪声。
func (p ModelPrice) moreExpensiveThan(other ModelPrice) bool {
	return p.Input+p.Output > other.Input+other.Output
}

// loadPrice 是缓存的回源函数：按 key 里的 (provider, model) 取一条单价。
//
// 未命中返回 (nil, nil)——由缓存层按 NullTTL 写空值标记（防穿透）。
func (c *Calculator) loadPrice(ctx context.Context, key string) (*ModelPrice, error) {
	provider, model, ok := splitPriceKey(key)
	if !ok {
		return nil, nil
	}
	p, err := c.repo.GetPricing(ctx, provider, model)
	if err != nil {
		return nil, err
	}
	if p == nil {
		return nil, nil
	}
	return &ModelPrice{
		Input:      p.InputPricePerMega,
		Output:     p.OutputPricePerMega,
		CacheRead:  p.CacheReadPricePerMega,
		CacheWrite: p.CacheWritePricePerMega,
	}, nil
}

// splitPriceKey 拆缓存 key "provider|model"。
func splitPriceKey(key string) (provider, model string, ok bool) {
	idx := strings.Index(key, "|")
	if idx < 0 {
		return "", "", false
	}
	return key[:idx], key[idx+1:], true
}

// Refresh 清空缓存，强制下次查询回源（供单价表变更后调用）。
func (c *Calculator) Refresh() {
	if c == nil || c.cache == nil {
		return
	}
	c.cache.Clear()
}

// SetPricing 写入/更新一条单价记录。
func (c *Calculator) SetPricing(ctx context.Context, p pricing.Pricing) error {
	if err := c.repo.SavePricing(ctx, p); err != nil {
		return err
	}
	c.Refresh()
	return nil
}

// ListPricing 列出所有单价记录。
func (c *Calculator) ListPricing(ctx context.Context) ([]pricing.Pricing, error) {
	return c.repo.ListPricing(ctx)
}

// DeletePricing 删除一条单价记录。
func (c *Calculator) DeletePricing(ctx context.Context, id uint) error {
	if err := c.repo.DeletePricing(ctx, id); err != nil {
		return err
	}
	c.Refresh()
	return nil
}
