package postgres

import (
	"context"
	"errors"
	"time"

	domainWebhook "github.com/yi-nology/code-review-service/internal/domain/webhook"
	"github.com/yi-nology/code-review-service/internal/repository/postgres/models"
	"github.com/yi-nology/code-review-service/internal/repository/repoerr"

	gormLib "gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type WebhookDeliveryRepo struct {
	db *gormLib.DB
}

func NewWebhookDeliveryRepo(db *gormLib.DB) *WebhookDeliveryRepo {
	return &WebhookDeliveryRepo{db: db}
}

func (r *WebhookDeliveryRepo) Create(ctx context.Context, d *domainWebhook.Delivery) error {
	m := deliveryToModel(d)
	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "provider"}, {Name: "installation_id"}, {Name: "delivery_id"}},
			DoNothing: true,
		}).
		Create(&m).Error
}

func (r *WebhookDeliveryRepo) Get(ctx context.Context, provider, installationID, deliveryID string) (*domainWebhook.Delivery, error) {
	var m models.WebhookDelivery
	err := r.db.WithContext(ctx).
		Where("provider = ? AND installation_id = ? AND delivery_id = ?", provider, installationID, deliveryID).
		First(&m).Error
	if err != nil {
		if errors.Is(err, gormLib.ErrRecordNotFound) {
			return nil, repoerr.ErrNotFound
		}
		return nil, err
	}
	d := deliveryToDomain(m)
	return &d, nil
}

func (r *WebhookDeliveryRepo) MarkProcessed(ctx context.Context, provider, installationID, deliveryID string, status, errMsg string) error {
	now := time.Now().UTC()
	updates := map[string]interface{}{
		"status":       status,
		"processed_at": now,
	}
	if errMsg != "" {
		updates["error"] = errMsg
	}
	return r.db.WithContext(ctx).
		Model(&models.WebhookDelivery{}).
		Where("provider = ? AND installation_id = ? AND delivery_id = ?", provider, installationID, deliveryID).
		Updates(updates).Error
}

func (r *WebhookDeliveryRepo) ListRecent(ctx context.Context, provider, status string, limit int) ([]domainWebhook.Delivery, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	db := r.db.WithContext(ctx).Model(&models.WebhookDelivery{})
	if provider != "" {
		db = db.Where("provider = ?", provider)
	}
	if status != "" {
		db = db.Where("status = ?", status)
	}
	var items []models.WebhookDelivery
	if err := db.Order("received_at DESC").Limit(limit).Find(&items).Error; err != nil {
		return nil, err
	}
	out := make([]domainWebhook.Delivery, len(items))
	for i, m := range items {
		out[i] = deliveryToDomain(m)
	}
	return out, nil
}

func (r *WebhookDeliveryRepo) CountByStatus(ctx context.Context, since time.Time) (map[string]int64, error) {
	type row struct {
		Status string
		Count  int64
	}
	db := r.db.WithContext(ctx).Model(&models.WebhookDelivery{})
	if !since.IsZero() {
		db = db.Where("received_at >= ?", since)
	}
	var rows []row
	if err := db.Select("status, COUNT(*) AS count").Group("status").Scan(&rows).Error; err != nil {
		return nil, err
	}
	out := make(map[string]int64, len(rows))
	for _, row := range rows {
		out[row.Status] = row.Count
	}
	return out, nil
}
