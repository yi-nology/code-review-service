package repository

import (
	"context"
	"time"

	domainWebhook "github.com/yi-nology/code-review-service/internal/domain/webhook"
)

// WebhookDeliveryRepository 已迁移为 domain 类型（#6）：不再依赖 postgres/models。
type WebhookDeliveryRepository interface {
	Create(ctx context.Context, d *domainWebhook.Delivery) error
	Get(ctx context.Context, provider, installationID, deliveryID string) (*domainWebhook.Delivery, error)
	MarkProcessed(ctx context.Context, provider, installationID, deliveryID, status, errMsg string) error
	ListRecent(ctx context.Context, provider, status string, limit int) ([]domainWebhook.Delivery, error)
	CountByStatus(ctx context.Context, since time.Time) (map[string]int64, error)
}
