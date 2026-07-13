package models

import "time"

// Notification 是站内通知记录（评审完成/失败/门禁未通过时生成）。
type Notification struct {
	ID        uint      `gorm:"primarykey" json:"id"`
	ReviewID  string    `gorm:"type:text;index:idx_notifications_review" json:"review_id"`
	Kind      string    `gorm:"type:text;not null" json:"kind"`
	Title     string    `gorm:"type:text;not null" json:"title"`
	Body      string    `gorm:"type:text" json:"body"`
	Read      bool      `gorm:"not null;default:false;index:idx_notifications_unread" json:"read"`
	Provider  string    `gorm:"type:text" json:"provider,omitempty"`
	Repo      string    `gorm:"type:text" json:"repo,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

func (Notification) TableName() string {
	return "notifications"
}
