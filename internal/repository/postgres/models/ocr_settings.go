package models

import (
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type OCRSettings struct {
	gorm.Model              `json:"-"`
	Enabled                 bool           `gorm:"not null;default:false" json:"enabled"`
	Binary                  string         `gorm:"type:text" json:"binary,omitempty"`
	Audience                string         `gorm:"type:text;default:'human'" json:"audience,omitempty"`
	DefaultSeverity         string         `gorm:"type:text;default:'major'" json:"default_severity,omitempty"`
	Provider                string         `gorm:"type:text" json:"provider,omitempty"`
	URL                     string         `gorm:"type:text" json:"url"`
	AuthToken               string         `gorm:"type:text" json:"auth_token,omitempty"`
	AuthHeader              string         `gorm:"type:text" json:"auth_header,omitempty"`
	ModelName               string         `gorm:"type:text;column:model" json:"model"`
	UseAnthropic            bool           `gorm:"not null;default:false" json:"use_anthropic"`
	ExtraBody               datatypes.JSON `gorm:"type:jsonb" json:"extra_body,omitempty"`
	Language                string         `gorm:"type:text;default:'Chinese'" json:"language"`
	Concurrency             int            `gorm:"not null;default:2" json:"concurrency"`
	MaxGitProcs             int            `gorm:"not null;default:4" json:"max_git_procs"`
	TimeoutMinutes          int            `gorm:"not null;default:10" json:"timeout_minutes"`
	MaxTools                int            `gorm:"not null;default:0" json:"max_tools"`
	Background              string         `gorm:"type:text" json:"background,omitempty"`
	ToolsPath               string         `gorm:"type:text" json:"tools_path,omitempty"`
	TelemetryEnabled        bool           `gorm:"not null;default:false" json:"telemetry_enabled"`
	TelemetryExporter       string         `gorm:"type:text" json:"telemetry_exporter,omitempty"`
	TelemetryOTLPEndpoint   string         `gorm:"type:text;column:telemetry_otlp_endpoint" json:"telemetry_otlp_endpoint,omitempty"`
	TelemetryContentLogging bool           `gorm:"not null;default:false" json:"telemetry_content_logging"`
}

func (OCRSettings) TableName() string {
	return "ocr_settings"
}
