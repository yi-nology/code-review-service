package models

import (
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type Review struct {
	gorm.Model         `json:"-"`
	ReviewID           string  `gorm:"type:text;uniqueIndex;not null" json:"review_id"`
	IdempotencyKey     *string `gorm:"type:text;uniqueIndex" json:"idempotency_key,omitempty"`
	RequestFingerprint string  `gorm:"type:text;not null;default:''" json:"request_fingerprint,omitempty"`
	Provider           string  `gorm:"type:text;not null;index:idx_provider_repo_status,priority:1" json:"provider"`
	Repo               string  `gorm:"type:text;not null;index:idx_provider_repo_status,priority:2" json:"repo"`
	Status             string  `gorm:"type:text;not null;index:idx_provider_repo_status,priority:3" json:"status"`
	Author             string  `gorm:"type:text;index" json:"author"`
	AuthorName         string  `gorm:"type:text" json:"author_name,omitempty"`
	AuthorAvatar       string  `gorm:"type:text" json:"author_avatar,omitempty"`
	Sender             string  `gorm:"type:text" json:"sender,omitempty"`

	PullRequestID   string `gorm:"type:text;index:idx_reviews_pr" json:"pull_request_id,omitempty"`
	MergeRequestIID string `gorm:"type:text;index:idx_reviews_mr" json:"merge_request_iid,omitempty"`
	BaseSHA         string `gorm:"type:text" json:"base_sha,omitempty"`
	HeadSHA         string `gorm:"type:text" json:"head_sha,omitempty"`

	Score   int    `gorm:"not null;default:0" json:"score"`
	Grade   string `gorm:"type:text" json:"grade,omitempty"`
	Summary string `gorm:"type:text" json:"summary,omitempty"`

	Request     datatypes.JSON `gorm:"type:jsonb;not null" json:"request"`
	Response    datatypes.JSON `gorm:"type:jsonb;not null" json:"response"`
	RawOCR      datatypes.JSON `gorm:"type:jsonb" json:"raw_ocr,omitempty"`
	RawOCRInput datatypes.JSON `gorm:"type:jsonb" json:"raw_ocr_input,omitempty"`
}

func (Review) TableName() string {
	return "reviews"
}
