package models

import (
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type OCRRulePackage struct {
	gorm.Model
	Name        string         `gorm:"type:text;not null;uniqueIndex" json:"name"`
	Description string         `gorm:"type:text" json:"description,omitempty"`
	Languages   datatypes.JSON `gorm:"type:jsonb" json:"languages,omitempty"`
	Tags        datatypes.JSON `gorm:"type:jsonb" json:"tags,omitempty"`
	Includes    datatypes.JSON `gorm:"type:jsonb" json:"includes,omitempty"`
	Excludes    datatypes.JSON `gorm:"type:jsonb" json:"excludes,omitempty"`
	Rules       datatypes.JSON `gorm:"type:jsonb;not null" json:"rules"`
	Enabled     bool           `gorm:"not null;default:true" json:"enabled"`
	ProjectRefs datatypes.JSON `gorm:"type:jsonb" json:"project_refs,omitempty"`
	BuiltIn     bool           `gorm:"not null;default:false" json:"built_in"`
	// Scope 区分规则集的合并来源：global=全局始终合并到每次审查；project=需被 agent/project 引用或 project_refs 匹配才生效。
	// 空值视为 "project"（向后兼容历史数据）。
	Scope string `gorm:"type:text;not null;default:'project'" json:"scope,omitempty"`
}

func (OCRRulePackage) TableName() string {
	return "ocr_rule_packages"
}
