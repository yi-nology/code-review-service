package postgres

import (
	"context"
	"time"

	"review-service/internal/repository/postgres/models"

	gormLib "gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type WebhookDeliveryRepo struct {
	db *gormLib.DB
}

func NewWebhookDeliveryRepo(db *gormLib.DB) *WebhookDeliveryRepo {
	return &WebhookDeliveryRepo{db: db}
}

func (r *WebhookDeliveryRepo) Create(ctx context.Context, d *models.WebhookDelivery) error {
	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "provider"}, {Name: "installation_id"}, {Name: "delivery_id"}},
			DoNothing: true,
		}).
		Create(d).Error
}

func (r *WebhookDeliveryRepo) Get(ctx context.Context, provider, installationID, deliveryID string) (*models.WebhookDelivery, error) {
	var d models.WebhookDelivery
	err := r.db.WithContext(ctx).
		Where("provider = ? AND installation_id = ? AND delivery_id = ?", provider, installationID, deliveryID).
		First(&d).Error
	if err != nil {
		if err == gormLib.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
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
