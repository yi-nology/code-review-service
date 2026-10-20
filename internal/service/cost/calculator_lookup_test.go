package cost

import (
	"context"
	"testing"

	"github.com/yi-nology/code-review-service/internal/domain/pricing"
	"github.com/yi-nology/code-review-service/internal/repository"
)

// fakePricingRepo 是 PricingRepository 的内存实现（仅本包测试用）。
type fakePricingRepo struct {
	rows []pricing.Pricing
}

func (f *fakePricingRepo) ListPricing(context.Context) ([]pricing.Pricing, error) {
	return f.rows, nil
}

func (f *fakePricingRepo) GetPricing(_ context.Context, provider, model string) (*pricing.Pricing, error) {
	for i := range f.rows {
		if f.rows[i].Provider == provider && f.rows[i].ModelName == model {
			p := f.rows[i]
			return &p, nil
		}
	}
	return nil, nil
}

func (f *fakePricingRepo) ListPricingByModel(_ context.Context, model string) ([]pricing.Pricing, error) {
	var out []pricing.Pricing
	for _, r := range f.rows {
		if r.ModelName == model {
			out = append(out, r)
		}
	}
	return out, nil
}

func (f *fakePricingRepo) SavePricing(_ context.Context, p pricing.Pricing) error {
	f.rows = append(f.rows, p)
	return nil
}

func (f *fakePricingRepo) DeletePricing(context.Context, uint) error { return nil }

var _ repository.PricingRepository = (*fakePricingRepo)(nil)

// TestEstimate_ProviderKnown 验证 provider 已知时按 (provider, model) 精确命中。
func TestEstimate_ProviderKnown(t *testing.T) {
	repo := &fakePricingRepo{rows: []pricing.Pricing{
		{Provider: "openai", ModelName: "gpt-4o", InputPricePerMega: 5, OutputPricePerMega: 15},
	}}
	c := NewCalculator(repo)
	// 1M input × $5/M + 1M output × $15/M = $20
	got := c.Estimate(context.Background(), "openai", "gpt-4o", 1_000_000, 1_000_000, 0, 0)
	if got != 20.0 {
		t.Errorf("Estimate = %v, want 20", got)
	}
}

// TestEstimate_ProviderUnknownFallsBackToModel 回归测试：
//
// provider 未知时必须按 model 名兜底命中单价。此前实现只有
// (provider|model) 一种键，provider 为空时拼出的 "|gpt-4o" 永远匹配不上
// 表里的 "openai|gpt-4o"——月度预算检查因此恒为 0，从不触发。
func TestEstimate_ProviderUnknownFallsBackToModel(t *testing.T) {
	repo := &fakePricingRepo{rows: []pricing.Pricing{
		{Provider: "openai", ModelName: "gpt-4o", InputPricePerMega: 5, OutputPricePerMega: 15},
	}}
	c := NewCalculator(repo)
	got := c.Estimate(context.Background(), "", "gpt-4o", 1_000_000, 1_000_000, 0, 0)
	if got != 20.0 {
		t.Errorf("provider-less lookup = %v, want 20（应按 model 兜底命中）", got)
	}
}

// TestEstimate_ProviderUnknownPicksMostExpensive 验证多 provider 同名模型时
// 取最贵的一条：预算是护栏，宁可高估也不因取到便宜的那条而放过超支。
func TestEstimate_ProviderUnknownPicksMostExpensive(t *testing.T) {
	repo := &fakePricingRepo{rows: []pricing.Pricing{
		{Provider: "cheap", ModelName: "gpt-4o", InputPricePerMega: 1, OutputPricePerMega: 2},
		{Provider: "pricey", ModelName: "gpt-4o", InputPricePerMega: 5, OutputPricePerMega: 15},
	}}
	c := NewCalculator(repo)
	got := c.Estimate(context.Background(), "", "gpt-4o", 1_000_000, 1_000_000, 0, 0)
	if got != 20.0 {
		t.Errorf("should pick the most expensive candidate, got %v, want 20", got)
	}
}

// TestEstimate_NoPriceAtAll 验证确实没有单价时返回 0（不阻断，仅缺数据）。
func TestEstimate_NoPriceAtAll(t *testing.T) {
	c := NewCalculator(&fakePricingRepo{})
	if got := c.Estimate(context.Background(), "openai", "unknown-model", 1000, 500, 0, 0); got != 0 {
		t.Errorf("no pricing should return 0, got %v", got)
	}
}
