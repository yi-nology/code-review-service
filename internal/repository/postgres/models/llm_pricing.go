package models

import (
	"time"

	"gorm.io/gorm"
)

// LLMModelPricing 记录每个 LLM 模型的 token 单价（每百万 token 美元）。
// 用于成本估算：review 的 input/output token × 对应单价 = 估算成本。
// 单价按 (provider, model) 唯一，可手动维护或从公开定价导入。
type LLMModelPricing struct {
	gorm.Model `json:"-"`
	// Provider 是 LLM 服务商标识（如 openai/anthropic/deepseek/local），
	// 对应 ocr_settings.provider 或 custom_provider 的 protocol。
	Provider string `gorm:"type:text;not null;uniqueIndex:idx_llm_pricing_unique,priority:1" json:"provider"`
	// ModelName 是模型名（如 gpt-4o/claude-3-5-sonnet/deepseek-chat），对应 ocr_settings.model。
	ModelName string `gorm:"column:model;type:text;not null;uniqueIndex:idx_llm_pricing_unique,priority:2" json:"model"`
	// InputPricePerMega 每百万 input token 单价（美元）。
	InputPricePerMega float64 `gorm:"type:numeric(12,6);not null;default:0" json:"input_price_per_mega"`
	// OutputPricePerMega 每百万 output token 单价（美元）。
	OutputPricePerMega float64 `gorm:"type:numeric(12,6);not null;default:0" json:"output_price_per_mega"`
	// CacheReadPricePerMega 每百万 cache-read token 单价（通常为 input 的折扣价，可空）。
	CacheReadPricePerMega float64 `gorm:"type:numeric(12,6);default:0" json:"cache_read_price_per_mega,omitempty"`
	// CacheWritePricePerMega 每百万 cache-write token 单价（通常略高于 input，可空）。
	CacheWritePricePerMega float64 `gorm:"type:numeric(12,6);default:0" json:"cache_write_price_per_mega,omitempty"`
	// Currency 货币单位，默认 USD。
	Currency string `gorm:"type:text;not null;default:'USD'" json:"currency"`
	// Notes 备注（如定价来源、生效日期）。
	Notes     string    `gorm:"type:text" json:"notes,omitempty"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (LLMModelPricing) TableName() string {
	return "llm_model_pricing"
}
