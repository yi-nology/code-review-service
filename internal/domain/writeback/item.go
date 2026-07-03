// Package writeback 是评审结果回写（评论/状态）的领域类型（#6 迁移）。
package writeback

import "time"

// Item 表示一条评审 finding 回写到 Git 平台的投递记录及其状态。
type Item struct {
	ReviewID           string     `json:"review_id"`
	FindingKey         string     `json:"finding_key"`
	Kind               string     `json:"kind"`
	ProviderExternalID string     `json:"provider_external_id,omitempty"`
	ProviderURL        string     `json:"provider_url,omitempty"`
	Status             string     `json:"status"`
	Attempts           int        `json:"attempts"`
	LastError          string     `json:"last_error,omitempty"`
	PublishedAt        *time.Time `json:"published_at,omitempty"`
}
