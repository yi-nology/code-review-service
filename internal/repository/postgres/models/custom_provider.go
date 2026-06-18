package models

import (
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type CustomProvider struct {
	gorm.Model `json:"-"`
	Name       string         `gorm:"type:text;not null;uniqueIndex" json:"name"`
	Protocol   string         `gorm:"type:text;not null" json:"protocol"`
	URL        string         `gorm:"type:text" json:"url,omitempty"`
	APIKey     string         `gorm:"type:text" json:"api_key,omitempty"`
	ModelName  string         `gorm:"type:text;column:model" json:"model,omitempty"`
	AuthHeader string         `gorm:"type:text" json:"auth_header,omitempty"`
	ExtraBody  datatypes.JSON `gorm:"type:jsonb" json:"extra_body,omitempty"`
	Enabled    bool           `gorm:"not null;default:true" json:"enabled"`
}

func (CustomProvider) TableName() string {
	return "custom_providers"
}
