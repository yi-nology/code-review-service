package models

import (
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type ReviewerAgent struct {
	gorm.Model  `json:"-"`
	Name        string `gorm:"type:text;uniqueIndex;not null" json:"name"`
	DisplayName string `gorm:"type:text;not null" json:"display_name"`
	Description string `gorm:"type:text" json:"description"`
	Background  string `gorm:"type:text" json:"background"`
	// RuleFile 是按 glob 文件模式的 rule JSON 内容，透传给 OCR 的 --rule（路径 A1）
	RuleFile string `gorm:"type:text" json:"rule_file,omitempty"`
	// Redundancy 是同一 persona 重复运行次数（路径 A4），默认 1
	Redundancy int  `gorm:"not null;default:1" json:"redundancy"`
	Enabled    bool `gorm:"not null;default:true" json:"enabled"`
	Priority   int  `gorm:"not null;default:0" json:"priority"`
	// IsDefault 标记该审查员属于「缺省集」：项目未配置 project_agents 且技术栈推断无匹配时，
	// 兜底运行 is_default=true 的审查员（避免空跑）。
	IsDefault bool           `gorm:"not null;default:false" json:"is_default"`
	Tags      datatypes.JSON `gorm:"type:jsonb" json:"tags,omitempty"`
}

func (ReviewerAgent) TableName() string {
	return "reviewer_agents"
}
