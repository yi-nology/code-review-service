package repository

import (
	"context"
	"time"

	"github.com/yi-nology/code-review-service/internal/repository/postgres/models"
)

type WebhookDeliveryRepository interface {
	Create(ctx context.Context, d *models.WebhookDelivery) error
	Get(ctx context.Context, provider, installationID, deliveryID string) (*models.WebhookDelivery, error)
	MarkProcessed(ctx context.Context, provider, installationID, deliveryID, status, errMsg string) error
	ListRecent(ctx context.Context, provider, status string, limit int) ([]models.WebhookDelivery, error)
	CountByStatus(ctx context.Context, since time.Time) (map[string]int64, error)
}
