package models

import (
	"time"

	"gorm.io/datatypes"
)

type WebhookDelivery struct {
	ID             uint           `gorm:"primarykey" json:"-"`
	Provider       string         `gorm:"type:text;not null;uniqueIndex:idx_webhook_delivery_unique" json:"provider"`
	InstallationID string         `gorm:"type:text;not null;uniqueIndex:idx_webhook_delivery_unique" json:"installation_id"`
	DeliveryID     string         `gorm:"type:text;not null;uniqueIndex:idx_webhook_delivery_unique" json:"delivery_id"`
	EventType      string         `gorm:"type:text;not null" json:"event_type"`
	Action         string         `gorm:"type:text;not null;default:''" json:"action"`
	PayloadSHA256  string         `gorm:"type:text;not null" json:"payload_sha256"`
	Payload        datatypes.JSON `gorm:"type:jsonb" json:"payload"`
	Status         string         `gorm:"type:text;not null;default:'received'" json:"status"`
	Error          string         `gorm:"type:text" json:"error,omitempty"`
	ReceivedAt     time.Time      `gorm:"not null" json:"received_at"`
	ProcessedAt    *time.Time     `json:"processed_at,omitempty"`
}

func (WebhookDelivery) TableName() string {
	return "webhook_deliveries"
}
