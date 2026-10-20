package postgres

import (
	"context"
	"errors"

	"github.com/yi-nology/code-review-service/internal/domain/pricing"
	"github.com/yi-nology/code-review-service/internal/repository/postgres/models"

	gormLib "gorm.io/gorm"
)

// PricingRepo 是 PricingRepository 的 PostgreSQL 实现。
type PricingRepo struct {
	db *gormLib.DB
}

func NewPricingRepo(db *gormLib.DB) *PricingRepo {
	return &PricingRepo{db: db}
}

// ListPricing 列出全部单价记录（按 provider, model 排序）。
func (r *PricingRepo) ListPricing(ctx context.Context) ([]pricing.Pricing, error) {
	var rows []models.LLMModelPricing
	if err := r.db.WithContext(ctx).Order("provider, model").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]pricing.Pricing, 0, len(rows))
	for _, m := range rows {
		out = append(out, pricingFromModel(m))
	}
	return out, nil
}

// GetPricing 按 (provider, model) 精确取一条；未命中返回 (nil, nil)。
func (r *PricingRepo) GetPricing(ctx context.Context, provider, model string) (*pricing.Pricing, error) {
	var row models.LLMModelPricing
	err := r.db.WithContext(ctx).
		Where("provider = ? AND model = ?", provider, model).
		First(&row).Error
	if err != nil {
		if errors.Is(err, gormLib.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	p := pricingFromModel(row)
	return &p, nil
}

// ListPricingByModel 按模型名取全部候选（provider 未知时的兜底）。
func (r *PricingRepo) ListPricingByModel(ctx context.Context, model string) ([]pricing.Pricing, error) {
	var rows []models.LLMModelPricing
	if err := r.db.WithContext(ctx).
		Where("model = ?", model).
		Order("id").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]pricing.Pricing, 0, len(rows))
	for _, m := range rows {
		out = append(out, pricingFromModel(m))
	}
	return out, nil
}

// SavePricing 写入或更新一条（带 ID 走更新，不带走插入）。
func (r *PricingRepo) SavePricing(ctx context.Context, p pricing.Pricing) error {
	m := pricingToModel(p)
	return r.db.WithContext(ctx).Save(&m).Error
}

// DeletePricing 删除一条。
func (r *PricingRepo) DeletePricing(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&models.LLMModelPricing{}, id).Error
}

// pricingToModel 把领域类型映射为 GORM 模型。
func pricingToModel(p pricing.Pricing) models.LLMModelPricing {
	return models.LLMModelPricing{
		// 保留主键：带 id 的请求走 Save 更新已有行；id=0 走插入。
		Model:                  gormLib.Model{ID: p.ID},
		Provider:               p.Provider,
		ModelName:              p.ModelName,
		InputPricePerMega:      p.InputPricePerMega,
		OutputPricePerMega:     p.OutputPricePerMega,
		CacheReadPricePerMega:  p.CacheReadPricePerMega,
		CacheWritePricePerMega: p.CacheWritePricePerMega,
		Currency:               p.Currency,
		Notes:                  p.Notes,
		UpdatedAt:              p.UpdatedAt,
	}
}

// pricingFromModel 把 GORM 模型映射为领域类型。
func pricingFromModel(m models.LLMModelPricing) pricing.Pricing {
	return pricing.Pricing{
		ID:                     m.ID,
		Provider:               m.Provider,
		ModelName:              m.ModelName,
		InputPricePerMega:      m.InputPricePerMega,
		OutputPricePerMega:     m.OutputPricePerMega,
		CacheReadPricePerMega:  m.CacheReadPricePerMega,
		CacheWritePricePerMega: m.CacheWritePricePerMega,
		Currency:               m.Currency,
		Notes:                  m.Notes,
		UpdatedAt:              m.UpdatedAt,
	}
}
