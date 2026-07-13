package models

import (
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type OCRSettings struct {
	gorm.Model      `json:"-"`
	Enabled         bool           `gorm:"not null;default:false" json:"enabled"`
	Binary          string         `gorm:"type:text" json:"binary,omitempty"`
	Audience        string         `gorm:"type:text;default:'agent'" json:"audience,omitempty"`
	DefaultSeverity string         `gorm:"type:text;default:'major'" json:"default_severity,omitempty"`
	Provider        string         `gorm:"type:text" json:"provider,omitempty"`
	URL             string         `gorm:"type:text" json:"url"`
	AuthToken       string         `gorm:"type:text" json:"auth_token,omitempty"`
	AuthHeader      string         `gorm:"type:text" json:"auth_header,omitempty"`
	ExtraHeaders    string         `gorm:"type:text" json:"extra_headers,omitempty"`
	ModelName       string         `gorm:"type:text;column:model" json:"model"`
	UseAnthropic    bool           `gorm:"not null;default:false" json:"use_anthropic"`
	ExtraBody       datatypes.JSON `gorm:"type:jsonb" json:"extra_body,omitempty"`
	// ReasoningMode 控制推理模型的行为：disabled / light / medium / full / custom（默认 light）。
	ReasoningMode string `gorm:"type:text;default:'light'" json:"reasoning_mode,omitempty"`
	// ReasoningBudget 自定义推理 token 预算，>0 时覆盖 ReasoningMode 的预设值（0=用预设）。
	ReasoningBudget         int            `gorm:"not null;default:0" json:"reasoning_budget,omitempty"`
	Language                string         `gorm:"type:text;default:'Chinese'" json:"language"`
	Concurrency             int            `gorm:"not null;default:2" json:"concurrency"`
	MaxGitProcs             int            `gorm:"not null;default:4" json:"max_git_procs"`
	TimeoutMinutes          int            `gorm:"not null;default:10" json:"timeout_minutes"`
	MaxTools                int            `gorm:"not null;default:0" json:"max_tools"`
	Background              string         `gorm:"type:text" json:"background,omitempty"`
	ToolsPath               string         `gorm:"type:text" json:"tools_path,omitempty"`
	ToolsConfig             datatypes.JSON `gorm:"type:jsonb" json:"tools_config,omitempty"`
	McpServers              datatypes.JSON `gorm:"type:jsonb" json:"mcp_servers,omitempty"`
	TelemetryEnabled        bool           `gorm:"not null;default:false" json:"telemetry_enabled"`
	TelemetryExporter       string         `gorm:"type:text" json:"telemetry_exporter,omitempty"`
	TelemetryOTLPEndpoint   string         `gorm:"type:text;column:telemetry_otlp_endpoint" json:"telemetry_otlp_endpoint,omitempty"`
	TelemetryContentLogging bool           `gorm:"not null;default:false" json:"telemetry_content_logging"`
	ContextWindow           int64          `gorm:"not null;default:0" json:"context_window"`
	MaxOutputTokens         int64          `gorm:"not null;default:0" json:"max_output_tokens"`
}

func (OCRSettings) TableName() string {
	return "ocr_settings"
}
