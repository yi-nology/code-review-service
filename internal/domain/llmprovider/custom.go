// Package llmprovider 是自定义 LLM provider 配置的领域类型（#6 迁移）。
// 命名为 llmprovider 以区别于 service/provider（Git 平台 provider）。
package llmprovider

import "encoding/json"

// Custom 表示一个用户自定义的 LLM 端点（OpenAI/Anthropic 兼容）。
// ExtraBody 为 JSONB，领域层用 json.RawMessage 解耦 GORM datatypes.JSON。
// ID 为持久化主键（update/delete-by-id 需要）。
type Custom struct {
	ID         uint            `json:"-"`
	Name       string          `json:"name"`
	Protocol   string          `json:"protocol"`
	URL        string          `json:"url,omitempty"`
	APIKey     string          `json:"api_key,omitempty"`
	ModelName  string          `json:"model,omitempty"`
	AuthHeader string          `json:"auth_header,omitempty"`
	ExtraBody  json.RawMessage `json:"extra_body,omitempty"`
	Enabled    bool            `json:"enabled"`
}
