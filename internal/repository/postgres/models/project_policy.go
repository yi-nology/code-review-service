package models

import (
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type ProjectPolicy struct {
	gorm.Model  `json:"-"`
	Provider    string         `gorm:"type:text;not null;uniqueIndex:idx_provider_repo" json:"provider"`
	Repo        string         `gorm:"type:text;not null;uniqueIndex:idx_provider_repo" json:"repo"`
	FailOn      string         `gorm:"type:text;not null;default:'critical'" json:"fail_on"`
	IgnorePaths datatypes.JSON `gorm:"type:jsonb" json:"ignore_paths"`
	Rules       datatypes.JSON `gorm:"type:jsonb" json:"rules"`
}

func (ProjectPolicy) TableName() string {
	return "project_policies"
}
