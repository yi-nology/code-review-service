// Package webhook 是 webhook 投递记录的领域类型（#6 迁移）。
package webhook

import (
	"encoding/json"
	"time"
)

// Delivery 表示一次入站 webhook 投递的记录与处理状态。
// Payload 为原始报文（JSONB），领域层用 json.RawMessage 解耦 GORM datatypes.JSON。
type Delivery struct {
	Provider       string          `json:"provider"`
	InstallationID string          `json:"installation_id"`
	DeliveryID     string          `json:"delivery_id"`
	EventType      string          `json:"event_type"`
	Action         string          `json:"action"`
	PayloadSHA256  string          `json:"payload_sha256"`
	Payload        json.RawMessage `json:"payload"`
	Status         string          `json:"status"`
	Error          string          `json:"error,omitempty"`
	ReceivedAt     time.Time       `json:"received_at"`
	ProcessedAt    *time.Time      `json:"processed_at,omitempty"`
}
