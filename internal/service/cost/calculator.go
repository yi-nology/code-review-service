// Package cost 实现 LLM 成本估算：根据 review 的 token 用量和模型单价计算估算成本，
// 并提供按 provider/项目/模型维度的成本统计与预算检查。
//
// 成本估算公式：input_tokens × input_price + output_tokens × output_price（每百万 token 单价）。
// 单价来自 llm_model_pricing 表（手动维护）；无单价的模型按 0 计算（不阻断，仅缺数据）。
package cost

import (
	"context"
	"sync"

	"github.com/yi-nology/code-review-service/internal/repository/postgres/models"
	"github.com/oekit/logx"

	"gorm.io/gorm"
)

// Calculator 计算 LLM 调用的估算成本。单价表缓存在内存，TTL 刷新。
type Calculator struct {
	db *gorm.DB

	mu    sync.RWMutex
	cache map[string]ModelPrice // key = provider|model
}

// ModelPrice 是单个模型的单价（每百万 token 美元）。
type ModelPrice struct {
	Input      float64
	Output     float64
	CacheRead  float64
	CacheWrite float64
}

func NewCalculator(db *gorm.DB) *Calculator {
	return &Calculator{db: db}
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

// lookup 查询单价（带内存缓存）。
func (c *Calculator) lookup(ctx context.Context, provider, model string) ModelPrice {
	key := provider + "|" + model
	c.mu.RLock()
	if c.cache != nil {
		if p, ok := c.cache[key]; ok {
			c.mu.RUnlock()
			return p
		}
	}
	c.mu.RUnlock()

	// 缓存未命中：若缓存未初始化或过期，重新加载全表。
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.cache == nil {
		c.reloadLocked(ctx)
	}
	if p, ok := c.cache[key]; ok {
		return p
	}
	// 该模型无单价记录：记一个零值占位，避免反复查库。
	if c.cache == nil {
		c.cache = map[string]ModelPrice{}
	}
	c.cache[key] = ModelPrice{}
	return ModelPrice{}
}

// reloadLocked 全量重载单价表到缓存。调用方须持有写锁。
func (c *Calculator) reloadLocked(ctx context.Context) {
	var rows []models.LLMModelPricing
	if err := c.db.WithContext(ctx).Find(&rows).Error; err != nil {
		logx.CtxWarnf(ctx, "[cost] reload pricing table failed: %v", err)
		return
	}
	m := make(map[string]ModelPrice, len(rows))
	for _, r := range rows {
		m[r.Provider+"|"+r.ModelName] = ModelPrice{
			Input:      r.InputPricePerMega,
			Output:     r.OutputPricePerMega,
			CacheRead:  r.CacheReadPricePerMega,
			CacheWrite: r.CacheWritePricePerMega,
		}
	}
	c.cache = m
}

// Refresh 清空缓存，强制下次查询重载（供单价表变更后调用）。
func (c *Calculator) Refresh() {
	c.mu.Lock()
	c.cache = nil
	c.mu.Unlock()
}

// SetPricing 写入/更新一条单价记录。
func (c *Calculator) SetPricing(ctx context.Context, p Pricing) error {
	m := p.toModel()
	if err := c.db.WithContext(ctx).Save(&m).Error; err != nil {
		return err
	}
	c.Refresh()
	return nil
}

// ListPricing 列出所有单价记录。
func (c *Calculator) ListPricing(ctx context.Context) ([]Pricing, error) {
	var rows []models.LLMModelPricing
	if err := c.db.WithContext(ctx).Order("provider, model").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]Pricing, 0, len(rows))
	for _, r := range rows {
		out = append(out, pricingFromModel(r))
	}
	return out, nil
}

// DeletePricing 删除一条单价记录。
func (c *Calculator) DeletePricing(ctx context.Context, id uint) error {
	if err := c.db.WithContext(ctx).Delete(&models.LLMModelPricing{}, id).Error; err != nil {
		return err
	}
	c.Refresh()
	return nil
}
