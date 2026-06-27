package models

import (
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type OCRToolPackage struct {
	gorm.Model
	Name        string         `gorm:"type:text;not null;uniqueIndex" json:"name"`
	Description string         `gorm:"type:text" json:"description,omitempty"`
	ScanType    string         `gorm:"type:text;not null;default:'all'" json:"scan_type"`
	Config      datatypes.JSON `gorm:"type:jsonb;not null" json:"config"`
	Enabled     bool           `gorm:"not null;default:true" json:"enabled"`
	ProjectRefs datatypes.JSON `gorm:"type:jsonb" json:"project_refs,omitempty"`
	BuiltIn     bool           `gorm:"not null;default:false" json:"built_in"`
}

func (OCRToolPackage) TableName() string {
	return "ocr_tool_packages"
}
