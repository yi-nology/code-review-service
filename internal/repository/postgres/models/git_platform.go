package models

import "gorm.io/gorm"

type GitPlatform struct {
	gorm.Model     `json:"-"`
	Provider       string `gorm:"type:text;uniqueIndex:idx_git_platforms_provider_install;not null" json:"provider"`
	InstallationID string `gorm:"type:text;uniqueIndex:idx_git_platforms_provider_install;not null;default:''" json:"installation_id"`
	BaseURL        string `gorm:"type:text;not null" json:"base_url"`
	Token          string `gorm:"type:text;not null" json:"-"`
	WebhookSecret  string `gorm:"type:text;column:webhook_secret" json:"webhook_secret,omitempty"`
	Enabled        bool   `gorm:"not null;default:true" json:"enabled"`
}

func (GitPlatform) TableName() string {
	return "git_platforms"
}

type GitTestResult struct {
	Connected bool   `json:"connected"`
	Platform  string `json:"platform"`
	UserName  string `json:"user_name"`
	Message   string `json:"message"`
}
