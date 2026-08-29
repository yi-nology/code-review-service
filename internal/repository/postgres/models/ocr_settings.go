package models

import (
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type OCRSettings struct {
	gorm.Model      `json:"-"`
	Enabled         bool   `gorm:"not null;default:false" json:"enabled"`
	Binary          string `gorm:"type:text" json:"binary,omitempty"`
	Audience        string `gorm:"type:text;default:'agent'" json:"audience,omitempty"`
	DefaultSeverity string `gorm:"type:text;default:'major'" json:"default_severity,omitempty"`
	Provider        string `gorm:"type:text" json:"provider,omitempty"`
	URL             string `gorm:"type:text" json:"url"`
	AuthToken       string `gorm:"type:text" json:"auth_token,omitempty"`
	AuthHeader      string `gorm:"type:text" json:"auth_header,omitempty"`
	ExtraHeaders    string `gorm:"type:text" json:"extra_headers,omitempty"`
	ModelName       string `gorm:"type:text;column:model" json:"model"`
	UseAnthropic    bool   `gorm:"not null;default:false" json:"use_anthropic"`
	// Protocol 控制 OCR 走哪种 LLM 协议：留空=由 UseAnthropic 推断（向后兼容）；
	// 可选值：anthropic / openai / openai-responses（GPT-5.x/o-series 必须用 openai-responses）。
	// 透传给 OCR 的 OCR_LLM_PROTOCOL 环境变量，优先级高于 UseAnthropic（OCR v1.7.10+）。
	Protocol string `gorm:"type:text" json:"protocol,omitempty"`
	// BackgroundFile 是 review/scan 的业务上下文 Markdown 文件路径（OCR v1.7.6+ --background-file）。
	// 与 Background 叠加：inline 内容在前，文件内容在后。文件需位于仓库工作区内或绝对白名单路径。
	BackgroundFile string         `gorm:"type:text" json:"background_file,omitempty"`
	ExtraBody      datatypes.JSON `gorm:"type:jsonb" json:"extra_body,omitempty"`
	// ReasoningMode 控制推理模型的行为：disabled / light / medium / full / custom（默认 light）。
	ReasoningMode string `gorm:"type:text;default:'light'" json:"reasoning_mode,omitempty"`
	// ReasoningBudget 自定义推理 token 预算，>0 时覆盖 ReasoningMode 的预设值（0=用预设）。
	ReasoningBudget         int            `gorm:"not null;default:0" json:"reasoning_budget,omitempty"`
	Language                string         `gorm:"type:text;default:'中文'" json:"language"`
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
	// 策略配置（原 policy.*，迁移到 DB）
	PolicyFailOn      string         `gorm:"type:text;default:'critical'" json:"policy_fail_on,omitempty"`
	PolicyMaxComments int            `gorm:"not null;default:20" json:"policy_max_comments,omitempty"`
	PolicySummaryOnly bool           `gorm:"not null;default:false" json:"policy_summary_only"`
	PolicyIgnorePaths datatypes.JSON `gorm:"type:jsonb" json:"policy_ignore_paths,omitempty"`
	// 写回配置
	MaxDiffBytes int64 `gorm:"not null;default:5242880" json:"max_diff_bytes,omitempty"`
}

func (OCRSettings) TableName() string {
	return "ocr_settings"
}
