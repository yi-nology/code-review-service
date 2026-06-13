package models

import (
	"time"
)

type ChangeRevision struct {
	ID             uint      `gorm:"primarykey" json:"-"`
	Provider       string    `gorm:"type:text;not null;uniqueIndex:idx_change_revision_unique" json:"provider"`
	InstallationID string    `gorm:"type:text;not null;uniqueIndex:idx_change_revision_unique" json:"installation_id"`
	RepositoryID   string    `gorm:"type:text;not null;uniqueIndex:idx_change_revision_unique" json:"repository_id"`
	Repository     string    `gorm:"type:text;not null" json:"repository"`
	ChangeNumber   int       `gorm:"not null;uniqueIndex:idx_change_revision_unique" json:"change_number"`
	BaseSHA        string    `gorm:"type:text;not null" json:"base_sha"`
	StartSHA       string    `gorm:"type:text;not null" json:"start_sha"`
	HeadSHA        string    `gorm:"type:text;not null;uniqueIndex:idx_change_revision_unique" json:"head_sha"`
	LatestReviewID string    `gorm:"type:text" json:"latest_review_id,omitempty"`
	CreatedAt      time.Time `gorm:"not null" json:"created_at"`
}

func (ChangeRevision) TableName() string {
	return "change_revisions"
}
