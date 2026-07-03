// Package ocrsettings 是 OCR 全局配置的领域类型（#6 迁移）。
package ocrsettings

import "encoding/json"

// Settings 是 OCR 引擎的全局配置（单例）。
// ExtraBody / ToolsConfig 为 JSONB，领域层用 json.RawMessage 解耦 GORM datatypes.JSON。
type Settings struct {
	Enabled                 bool            `json:"enabled"`
	Binary                  string          `json:"binary,omitempty"`
	Audience                string          `json:"audience,omitempty"`
	DefaultSeverity         string          `json:"default_severity,omitempty"`
	Provider                string          `json:"provider,omitempty"`
	URL                     string          `json:"url"`
	AuthToken               string          `json:"auth_token,omitempty"`
	AuthHeader              string          `json:"auth_header,omitempty"`
	ExtraHeaders            string          `json:"extra_headers,omitempty"`
	ModelName               string          `json:"model"`
	UseAnthropic            bool            `json:"use_anthropic"`
	ExtraBody               json.RawMessage `json:"extra_body,omitempty"`
	ReasoningMode           string          `json:"reasoning_mode,omitempty"`
	ReasoningBudget         int             `json:"reasoning_budget,omitempty"`
	Language                string          `json:"language"`
	Concurrency             int             `json:"concurrency"`
	MaxGitProcs             int             `json:"max_git_procs"`
	TimeoutMinutes          int             `json:"timeout_minutes"`
	MaxTools                int             `json:"max_tools"`
	Background              string          `json:"background,omitempty"`
	ToolsPath               string          `json:"tools_path,omitempty"`
	ToolsConfig             json.RawMessage `json:"tools_config,omitempty"`
	TelemetryEnabled        bool            `json:"telemetry_enabled"`
	TelemetryExporter       string          `json:"telemetry_exporter,omitempty"`
	TelemetryOTLPEndpoint   string          `json:"telemetry_otlp_endpoint,omitempty"`
	TelemetryContentLogging bool            `json:"telemetry_content_logging"`
	ContextWindow           int64           `json:"context_window"`
	MaxOutputTokens         int64           `json:"max_output_tokens"`
}
