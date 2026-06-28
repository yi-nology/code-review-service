package models

import "time"

type FindingFeedback struct {
	ID           uint      `gorm:"primarykey" json:"-"`
	ReviewID     string    `gorm:"type:text;not null;uniqueIndex:idx_finding_feedback_unique" json:"review_id"`
	FindingIndex int       `gorm:"not null;uniqueIndex:idx_finding_feedback_unique" json:"finding_index"`
	Status       string    `gorm:"type:text;not null" json:"status"`
	Note         string    `gorm:"type:text" json:"note,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func (FindingFeedback) TableName() string {
	return "finding_feedback"
}
