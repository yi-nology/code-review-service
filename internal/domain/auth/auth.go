// Package auth 定义 API Token 与审计日志的领域类型。
//
// 多 Token 方案（替代单一 REVIEW_TOKEN）：每个 token 具名、带 scope、可审计。
// 这是 POC 推广阶段的过渡方案，比完整 RBAC/SSO 轻量，但解决了「谁做了什么」的追溯问题。
package auth

import "time"

// Token 是一条具名 API Token 的领域表示。
type Token struct {
	ID         uint
	Name       string
	TokenHash  string // SHA256 哈希，不暴露明文
	Scopes     []string
	CreatedBy  string
	LastUsedAt *time.Time
	ExpiresAt  *time.Time
	CreatedAt  time.Time
}

// AuditEntry 是一条审计日志记录。
type AuditEntry struct {
	ID         uint
	TokenName  string
	Action     string // 如 "settings.platform.delete"
	Resource   string
	ResourceID string
	Method     string
	Path       string
	StatusCode int
	Detail     string
	CreatedAt  time.Time
}
