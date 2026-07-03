// Package projectpolicy 是项目级评审策略的领域类型（#6 迁移）。
// 命名为 projectpolicy 以区别于 service/policy（策略引擎）。
package projectpolicy

import "encoding/json"

// ProjectPolicy 是某项目（provider+repo）的评审策略与 OCR 覆盖配置。
// IgnorePaths/Rules 为 JSONB，领域层用 json.RawMessage 解耦 GORM datatypes.JSON。
type ProjectPolicy struct {
	Provider                   string          `json:"provider"`
	Repo                       string          `json:"repo"`
	FailOn                     string          `json:"fail_on"`
	IgnorePaths                json.RawMessage `json:"ignore_paths"`
	Rules                      json.RawMessage `json:"rules"`
	OCREnabled                 *bool           `json:"ocr_enabled,omitempty"`
	OCRConcurrency             int             `json:"ocr_concurrency,omitempty"`
	OCRTimeoutMinutes          int             `json:"ocr_timeout_minutes,omitempty"`
	OCRMaxTools                int             `json:"ocr_max_tools,omitempty"`
	OCRDefaultSeverity         string          `json:"ocr_default_severity,omitempty"`
	OCRBackground              string          `json:"ocr_background,omitempty"`
	OCRAudience                string          `json:"ocr_audience,omitempty"`
	OCRProvider                string          `json:"ocr_provider,omitempty"`
	OCRURL                     string          `json:"ocr_url,omitempty"`
	OCRModel                   string          `json:"ocr_model,omitempty"`
	OCRMaxGitProcs             int             `json:"ocr_max_git_procs,omitempty"`
	OCRUseAnthropic            bool            `json:"ocr_use_anthropic"`
	OCRExtraBody               string          `json:"ocr_extra_body,omitempty"`
	OCRLanguage                string          `json:"ocr_language,omitempty"`
	OCRBinary                  string          `json:"ocr_binary,omitempty"`
	OCRToolsPath               string          `json:"ocr_tools_path,omitempty"`
	OCRTelemetryEnabled        bool            `json:"ocr_telemetry_enabled"`
	OCRTelemetryExporter       string          `json:"ocr_telemetry_exporter,omitempty"`
	OCRTelemetryOTLPEndpoint   string          `json:"ocr_telemetry_otlp_endpoint,omitempty"`
	OCRTelemetryContentLogging bool            `json:"ocr_telemetry_content_logging"`
}
