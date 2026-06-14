package models

import (
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type OCRSettings struct {
	gorm.Model     `json:"-"`
	Enabled        bool           `gorm:"not null;default:false" json:"enabled"`
	Provider       string         `gorm:"type:text" json:"provider,omitempty"`
	Plan           string         `gorm:"type:text" json:"plan,omitempty"`
	URL            string         `gorm:"type:text" json:"url"`
	AuthToken      string         `gorm:"type:text" json:"auth_token,omitempty"`
	AuthHeader     string         `gorm:"type:text" json:"auth_header,omitempty"`
	ModelName      string         `gorm:"type:text;column:model" json:"model"`
	UseAnthropic   bool           `gorm:"not null;default:false" json:"use_anthropic"`
	ExtraBody      datatypes.JSON `gorm:"type:jsonb" json:"extra_body,omitempty"`
	Language       string         `gorm:"type:text;default:'Chinese'" json:"language"`
	Concurrency    int            `gorm:"not null;default:2" json:"concurrency"`
	MaxGitProcs    int            `gorm:"not null;default:4" json:"max_git_procs"`
	TimeoutMinutes int            `gorm:"not null;default:10" json:"timeout_minutes"`
	MaxTools       int            `gorm:"not null;default:0" json:"max_tools"`
	Background     string         `gorm:"type:text" json:"background,omitempty"`
}

func (OCRSettings) TableName() string {
	return "ocr_settings"
}
