package postgres

import (
	"context"
	"strings"
	"time"

	"github.com/yi-nology/code-review-service/internal/domain/auth"
	"github.com/yi-nology/code-review-service/internal/repository/postgres/models"

	gormLib "gorm.io/gorm"
)

type AuthRepo struct {
	db *gormLib.DB
}

func NewAuthRepo(db *gormLib.DB) *AuthRepo {
	return &AuthRepo{db: db}
}

func (r *AuthRepo) CreateToken(ctx context.Context, t *auth.Token) error {
	m := tokenToModel(t)
	if err := r.db.WithContext(ctx).Create(&m).Error; err != nil {
		return err
	}
	t.ID = m.ID
	t.CreatedAt = m.CreatedAt
	return nil
}

func (r *AuthRepo) GetTokenByHash(ctx context.Context, hash string) (*auth.Token, error) {
	var m models.APIToken
	err := r.db.WithContext(ctx).Where("token_hash = ?", hash).First(&m).Error
	if err != nil {
		return nil, err
	}
	t := tokenToDomain(m)
	return &t, nil
}

func (r *AuthRepo) ListTokens(ctx context.Context) ([]auth.Token, error) {
	var rows []models.APIToken
	if err := r.db.WithContext(ctx).Order("created_at DESC").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]auth.Token, len(rows))
	for i, m := range rows {
		out[i] = tokenToDomain(m)
	}
	return out, nil
}

func (r *AuthRepo) DeleteToken(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&models.APIToken{}, id).Error
}

func (r *AuthRepo) UpdateLastUsed(ctx context.Context, hash string, at interface{}) error {
	t, ok := at.(time.Time)
	if !ok {
		t = time.Now()
	}
	return r.db.WithContext(ctx).Model(&models.APIToken{}).
		Where("token_hash = ?", hash).Update("last_used_at", t).Error
}

func (r *AuthRepo) WriteAudit(ctx context.Context, entry *auth.AuditEntry) error {
	m := auditToModel(entry)
	if err := r.db.WithContext(ctx).Create(&m).Error; err != nil {
		return err
	}
	entry.ID = m.ID
	return nil
}

func (r *AuthRepo) ListAudit(ctx context.Context, limit, offset int) ([]auth.AuditEntry, int64, error) {
	if limit <= 0 {
		limit = 50
	}
	var total int64
	if err := r.db.WithContext(ctx).Model(&models.AuditLog{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []models.AuditLog
	if err := r.db.WithContext(ctx).Order("created_at DESC").Limit(limit).Offset(offset).Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	out := make([]auth.AuditEntry, len(rows))
	for i, m := range rows {
		out[i] = auditToDomain(m)
	}
	return out, total, nil
}

func tokenToModel(t *auth.Token) models.APIToken {
	return models.APIToken{
		Name: t.Name, TokenHash: t.TokenHash, Scopes: strings.Join(t.Scopes, ","),
		CreatedBy: t.CreatedBy, LastUsedAt: t.LastUsedAt, ExpiresAt: t.ExpiresAt,
	}
}

func tokenToDomain(m models.APIToken) auth.Token {
	var scopes []string
	if m.Scopes != "" {
		scopes = strings.Split(m.Scopes, ",")
	}
	return auth.Token{
		ID: m.ID, Name: m.Name, TokenHash: m.TokenHash, Scopes: scopes,
		CreatedBy: m.CreatedBy, LastUsedAt: m.LastUsedAt, ExpiresAt: m.ExpiresAt,
		CreatedAt: m.CreatedAt,
	}
}

func auditToModel(e *auth.AuditEntry) models.AuditLog {
	return models.AuditLog{
		TokenName: e.TokenName, Action: e.Action, Resource: e.Resource, ResourceID: e.ResourceID,
		Method: e.Method, Path: e.Path, StatusCode: e.StatusCode, Detail: e.Detail,
	}
}

func auditToDomain(m models.AuditLog) auth.AuditEntry {
	return auth.AuditEntry{
		ID: m.ID, TokenName: m.TokenName, Action: m.Action, Resource: m.Resource,
		ResourceID: m.ResourceID, Method: m.Method, Path: m.Path,
		StatusCode: m.StatusCode, Detail: m.Detail, CreatedAt: m.CreatedAt,
	}
}
