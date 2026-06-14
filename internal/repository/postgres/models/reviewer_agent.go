package models

import (
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type ReviewerAgent struct {
	gorm.Model     `json:"-"`
	Name           string         `gorm:"type:text;uniqueIndex;not null" json:"name"`
	DisplayName    string         `gorm:"type:text;not null" json:"display_name"`
	Description    string         `gorm:"type:text" json:"description"`
	Background     string         `gorm:"type:text" json:"background"`
	Enabled        bool           `gorm:"not null;default:true" json:"enabled"`
	Priority       int            `gorm:"not null;default:0" json:"priority"`
	Tags           datatypes.JSON `gorm:"type:jsonb" json:"tags,omitempty"`
}

func (ReviewerAgent) TableName() string {
	return "reviewer_agents"
}
