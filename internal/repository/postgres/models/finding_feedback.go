package models

import "time"

type FindingFeedback struct {
	ID           uint   `gorm:"primarykey" json:"-"`
	ReviewID     string `gorm:"type:text;not null;uniqueIndex:idx_finding_feedback_unique" json:"review_id"`
	FindingIndex int    `gorm:"not null;uniqueIndex:idx_finding_feedback_unique" json:"finding_index"`
	// Fingerprint 是 finding 的跨 review 稳定指纹（可空，向后兼容历史数据）。
	// 加普通索引供误报抑制查询（按指纹查历史反馈），非唯一约束。
	Fingerprint string    `gorm:"type:text;index:idx_finding_feedback_fingerprint" json:"fingerprint,omitempty"`
	Status      string    `gorm:"type:text;not null" json:"status"`
	Note        string    `gorm:"type:text" json:"note,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (FindingFeedback) TableName() string {
	return "finding_feedback"
}
