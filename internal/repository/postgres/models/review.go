package models

import (
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type Review struct {
	gorm.Model         `json:"-"`
	ReviewID           string         `gorm:"type:text;uniqueIndex;not null" json:"review_id"`
	IdempotencyKey     *string        `gorm:"type:text;uniqueIndex" json:"idempotency_key,omitempty"`
	RequestFingerprint string         `gorm:"type:text;not null;default:''" json:"request_fingerprint,omitempty"`
	Provider           string         `gorm:"type:text;not null;index;index:idx_provider_repo_status,priority:1" json:"provider"`
	Repo               string         `gorm:"type:text;not null;index;index:idx_provider_repo_status,priority:2" json:"repo"`
	Status             string         `gorm:"type:text;not null;index;index:idx_provider_repo_status,priority:3" json:"status"`
	Author             string         `gorm:"type:text;index" json:"author"`
	AuthorName         string         `gorm:"type:text" json:"author_name,omitempty"`
	AuthorAvatar       string         `gorm:"type:text" json:"author_avatar,omitempty"`
	Sender             string         `gorm:"type:text" json:"sender,omitempty"`
	Request            datatypes.JSON `gorm:"type:jsonb;not null" json:"request"`
	Response           datatypes.JSON `gorm:"type:jsonb;not null" json:"response"`
	RawOCR             datatypes.JSON `gorm:"type:jsonb" json:"raw_ocr,omitempty"`
}

func (Review) TableName() string {
	return "reviews"
}
