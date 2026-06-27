package models

import (
	"time"

	"gorm.io/datatypes"
)

type OCRRulePackageVersion struct {
	ID        uint           `gorm:"primarykey" json:"id"`
	PackageID uint           `gorm:"not null;index;uniqueIndex:idx_ocr_rule_package_version" json:"package_id"`
	Version   int            `gorm:"not null;uniqueIndex:idx_ocr_rule_package_version" json:"version"`
	Snapshot  datatypes.JSON `gorm:"type:jsonb;not null" json:"snapshot"`
	CreatedAt time.Time      `json:"created_at"`
}

func (OCRRulePackageVersion) TableName() string {
	return "ocr_rule_package_versions"
}

type OCRToolPackageVersion struct {
	ID        uint           `gorm:"primarykey" json:"id"`
	PackageID uint           `gorm:"not null;index;uniqueIndex:idx_ocr_tool_package_version" json:"package_id"`
	Version   int            `gorm:"not null;uniqueIndex:idx_ocr_tool_package_version" json:"version"`
	Snapshot  datatypes.JSON `gorm:"type:jsonb;not null" json:"snapshot"`
	CreatedAt time.Time      `json:"created_at"`
}

func (OCRToolPackageVersion) TableName() string {
	return "ocr_tool_package_versions"
}
