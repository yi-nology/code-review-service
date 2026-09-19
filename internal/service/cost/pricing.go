package cost

import (
	"time"

	"github.com/yi-nology/code-review-service/internal/repository/postgres/models"

	"gorm.io/gorm"
)

// Pricing 是模型单价的业务/API 类型。handler 绑定本类型，
// GORM 模型（models.LLMModelPricing）只在 Calculator 内部映射时出现，
// 避免 HTTP 层直接耦合 DB schema。
//
// JSON 字段与 models.LLMModelPricing 的 json tag 兼容，另补 id 字段：
// 列表响应带 id，前端据此调用 DELETE /pricing/:id 与带 id 的更新。
type Pricing struct {
	ID uint `json:"id,omitempty"`

	Provider               string    `json:"provider"`
	ModelName              string    `json:"model"`
	InputPricePerMega      float64   `json:"input_price_per_mega"`
	OutputPricePerMega     float64   `json:"output_price_per_mega"`
	CacheReadPricePerMega  float64   `json:"cache_read_price_per_mega,omitempty"`
	CacheWritePricePerMega float64   `json:"cache_write_price_per_mega,omitempty"`
	Currency               string    `json:"currency"`
	Notes                  string    `json:"notes,omitempty"`
	UpdatedAt              time.Time `json:"updated_at"`
}

func (p Pricing) toModel() models.LLMModelPricing {
	return models.LLMModelPricing{
		// 保留主键：带 id 的请求走 Save 更新已有行；id=0 走插入。
		Model:                  gorm.Model{ID: p.ID},
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

func pricingFromModel(m models.LLMModelPricing) Pricing {
	return Pricing{
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
