package postgres

import (
	"context"
	"time"

	"github.com/yi-nology/code-review-service/internal/repository/postgres/models"

	gormLib "gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type WritebackItemRepo struct {
	db *gormLib.DB
}

func NewWritebackItemRepo(db *gormLib.DB) *WritebackItemRepo {
	return &WritebackItemRepo{db: db}
}

func (r *WritebackItemRepo) Create(ctx context.Context, item *models.WritebackItem) error {
	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "review_id"}, {Name: "finding_key"}, {Name: "kind"}},
			DoNothing: true,
		}).
		Create(item).Error
}

func (r *WritebackItemRepo) UpdateStatus(ctx context.Context, reviewID, findingKey, kind, status, externalID, providerURL, lastErr string) error {
	updates := map[string]interface{}{
		"status":     status,
		"last_error": lastErr,
	}
	if externalID != "" {
		updates["provider_external_id"] = externalID
	}
	if providerURL != "" {
		updates["provider_url"] = providerURL
	}
	if status == "published" {
		now := time.Now().UTC()
		updates["published_at"] = &now
	}
	return r.db.WithContext(ctx).
		Model(&models.WritebackItem{}).
		Where("review_id = ? AND finding_key = ? AND kind = ?", reviewID, findingKey, kind).
		Updates(updates).Error
}

func (r *WritebackItemRepo) IncrementAttempts(ctx context.Context, reviewID, findingKey, kind string) error {
	return r.db.WithContext(ctx).
		Model(&models.WritebackItem{}).
		Where("review_id = ? AND finding_key = ? AND kind = ?", reviewID, findingKey, kind).
		UpdateColumn("attempts", gormLib.Expr("attempts + 1")).Error
}

func (r *WritebackItemRepo) ListByReview(ctx context.Context, reviewID string) ([]models.WritebackItem, error) {
	var items []models.WritebackItem
	err := r.db.WithContext(ctx).
		Where("review_id = ?", reviewID).
		Find(&items).Error
	return items, err
}

func (r *WritebackItemRepo) ListRetryable(ctx context.Context, limit int) ([]models.WritebackItem, error) {
	var items []models.WritebackItem
	err := r.db.WithContext(ctx).
		Where("status IN ? AND attempts < ?", []string{"pending", "failed"}, 5).
		Order("published_at").
		Limit(limit).
		Find(&items).Error
	return items, err
}
