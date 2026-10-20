package repository

import (
	"context"

	"github.com/yi-nology/code-review-service/internal/domain/pricing"
)

// PricingRepository 持久化 LLM 模型单价表。
//
// 单价表的唯一键是 (provider, model)——同一模型名可出现在多个 provider 下。
type PricingRepository interface {
	// ListPricing 列出全部单价记录（按 provider, model 排序）。
	ListPricing(ctx context.Context) ([]pricing.Pricing, error)
	// GetPricing 按 (provider, model) 精确取一条；未命中返回 (nil, nil)。
	GetPricing(ctx context.Context, provider, model string) (*pricing.Pricing, error)
	// ListPricingByModel 按模型名取全部候选。
	//
	// 用于 provider 未知时的兜底（月度用量只按 model 聚合，拿不到 provider）。
	// 同一模型名在多个 provider 下时返回多条，由调用方定取舍口径。
	ListPricingByModel(ctx context.Context, model string) ([]pricing.Pricing, error)
	// SavePricing 写入或更新一条（带 ID 走更新，不带走插入）。
	SavePricing(ctx context.Context, p pricing.Pricing) error
	// DeletePricing 删除一条。
	DeletePricing(ctx context.Context, id uint) error
}
