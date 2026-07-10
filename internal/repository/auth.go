package repository

import (
	"context"

	"github.com/yi-nology/code-review-service/internal/domain/auth"
)

// AuthRepository 管理 API Token 与审计日志。
type AuthRepository interface {
	// Token 操作
	CreateToken(ctx context.Context, t *auth.Token) error
	// GetTokenByHash 按哈希查 token（鉴权中间件用）。
	GetTokenByHash(ctx context.Context, hash string) (*auth.Token, error)
	ListTokens(ctx context.Context) ([]auth.Token, error)
	DeleteToken(ctx context.Context, id uint) error
	UpdateLastUsed(ctx context.Context, hash string, at interface{}) error

	// 审计日志
	WriteAudit(ctx context.Context, entry *auth.AuditEntry) error
	ListAudit(ctx context.Context, limit, offset int) ([]auth.AuditEntry, int64, error)
}
