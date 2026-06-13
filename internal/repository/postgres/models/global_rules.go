package models

import (
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type GlobalRules struct {
	gorm.Model `json:"-"`
	Rules      datatypes.JSON `gorm:"type:jsonb" json:"rules"`
}

func (GlobalRules) TableName() string {
	return "global_rules"
}
