// Package rules 是全局评审规则的领域类型（#6 迁移）。
package rules

import "encoding/json"

// Global 是全量评审规则的单例（JSON 规则集合）。
// Content 为 JSONB，领域层用 json.RawMessage 解耦 GORM datatypes.JSON。
type Global struct {
	Content json.RawMessage `json:"rules"`
}
