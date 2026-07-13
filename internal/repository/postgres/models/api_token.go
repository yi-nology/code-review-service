package models

import (
	"time"

	"gorm.io/gorm"
)

// APIToken 是具名 API Token（替代单一全局 REVIEW_TOKEN）。
// 支持多 token（如「张三-web」「CI-bot」），每个 token 带 scope 和命名，便于审计。
// token 以 SHA256 哈希存储（不存明文），调用方只在创建时拿到一次明文。
type APIToken struct {
	gorm.Model `json:"-"`
	Name       string `gorm:"type:text;not null" json:"name"`
	TokenHash  string `gorm:"type:text;not null;uniqueIndex:idx_api_tokens_hash" json:"-"`
	Scopes     string `gorm:"type:text;not null;default:''" json:"scopes,omitempty"` // 逗号分隔，如 "review:write,settings:read"
	CreatedBy  string `gorm:"type:text" json:"created_by,omitempty"`
	// LastUsedAt 记录最近一次使用时间，供审计/清理。
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
	// ExpiresAt 为空表示永不过期。
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
}

func (APIToken) TableName() string {
	return "api_tokens"
}

// AuditLog 记录关键写操作的审计轨迹（谁、何时、做了什么）。
// 用于多 token 场景下追溯「哪个 token 改了配置 / 应用了补丁 / 删除了平台」。
type AuditLog struct {
	gorm.Model `json:"-"`
	TokenName  string    `gorm:"type:text;index:idx_audit_token" json:"token_name"`
	Action     string    `gorm:"type:text;not null;index:idx_audit_action" json:"action"`
	Resource   string    `gorm:"type:text" json:"resource,omitempty"`
	ResourceID string    `gorm:"type:text" json:"resource_id,omitempty"`
	Method     string    `gorm:"type:text" json:"method"`
	Path       string    `gorm:"type:text" json:"path"`
	StatusCode int       `json:"status_code"`
	Detail     string    `gorm:"type:text" json:"detail,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}

func (AuditLog) TableName() string {
	return "audit_logs"
}
