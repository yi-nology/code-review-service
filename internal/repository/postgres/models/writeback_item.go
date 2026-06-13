package models

import (
	"time"
)

type WritebackItem struct {
	ID                 uint      `gorm:"primarykey" json:"-"`
	ReviewID           string    `gorm:"type:text;not null;uniqueIndex:idx_writeback_item_unique" json:"review_id"`
	FindingKey         string    `gorm:"type:text;not null;uniqueIndex:idx_writeback_item_unique" json:"finding_key"`
	Kind               string    `gorm:"type:text;not null;uniqueIndex:idx_writeback_item_unique" json:"kind"`
	ProviderExternalID string    `gorm:"type:text" json:"provider_external_id,omitempty"`
	ProviderURL        string    `gorm:"type:text" json:"provider_url,omitempty"`
	Status             string    `gorm:"type:text;not null;default:'pending'" json:"status"`
	Attempts           int       `gorm:"not null;default:0" json:"attempts"`
	LastError          string    `gorm:"type:text" json:"last_error,omitempty"`
	PublishedAt        *time.Time `json:"published_at,omitempty"`
}

func (WritebackItem) TableName() string {
	return "writeback_items"
}
