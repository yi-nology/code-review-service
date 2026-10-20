// Package pricing 定义模型单价的领域类型。
//
// 单价表是业务资产（管理员在 Web 控制台维护），但它既被 HTTP 层直接绑定，
// 也被成本估算的热路径读取。此前该类型住在 internal/service/cost，
// 于是持久化层要反向依赖 service 包——infra → service 的方向是倒的。
//
// 移到这里后：HTTP 层仍可绑本类型（JSON 字段不变），仓储层依赖本包，
// service 层也依赖本包，三方都不再互相依赖。
package pricing

import "time"

// Pricing 是模型单价的业务/API 类型。
//
// JSON 字段与 GORM 模型 models.LLMModelPricing 的 json tag 兼容，另补 id
// 字段：列表响应带 id，前端据此调用 DELETE /pricing/:id 与带 id 的更新。
//
// 单价单位是「每百万 token 美元」。
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
