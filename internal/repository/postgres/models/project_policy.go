package models

import (
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type ProjectPolicy struct {
	gorm.Model `json:"-"`
	Provider   string `gorm:"type:text;not null;uniqueIndex:idx_provider_repo" json:"provider"`
	Repo       string `gorm:"type:text;not null;uniqueIndex:idx_provider_repo" json:"repo"`
	FailOn     string `gorm:"type:text;not null;default:'critical'" json:"fail_on"`
	// ReviewEngine 该项目的默认审查引擎：''（默认 OCR）| 'ocr' | 'argus'。
	// webhook 创建评审时按此路由；请求显式指定 engine 时优先于本字段。
	ReviewEngine       string         `gorm:"type:text;not null;default:''" json:"review_engine,omitempty"`
	IgnorePaths        datatypes.JSON `gorm:"type:jsonb" json:"ignore_paths"`
	Rules              datatypes.JSON `gorm:"type:jsonb" json:"rules"`
	OCREnabled         *bool          `gorm:"type:boolean" json:"ocr_enabled,omitempty"`
	OCRConcurrency     int            `gorm:"not null;default:0" json:"ocr_concurrency,omitempty"`
	OCRTimeoutMinutes  int            `gorm:"not null;default:0" json:"ocr_timeout_minutes,omitempty"`
	OCRMaxTools        int            `gorm:"not null;default:0" json:"ocr_max_tools,omitempty"`
	OCRDefaultSeverity string         `gorm:"type:text" json:"ocr_default_severity,omitempty"`
	OCRBackground      string         `gorm:"type:text" json:"ocr_background,omitempty"`
	// OCRBackgroundFile 项目级 --background-file 路径覆盖（OCR v1.7.6+）；留空=回退到全局。
	OCRBackgroundFile string `gorm:"type:text" json:"ocr_background_file,omitempty"`
	OCRAudience       string `gorm:"type:text" json:"ocr_audience,omitempty"`
	OCRProvider       string `gorm:"type:text" json:"ocr_provider,omitempty"`
	OCRURL            string `gorm:"type:text" json:"ocr_url,omitempty"`
	OCRModel          string `gorm:"type:text" json:"ocr_model,omitempty"`
	OCRMaxGitProcs    int    `gorm:"not null;default:0" json:"ocr_max_git_procs,omitempty"`
	OCRUseAnthropic   bool   `gorm:"not null;default:false" json:"ocr_use_anthropic"`
	// OCRProtocol 项目级覆盖 OCR 协议（anthropic / openai / openai-responses）；留空=回退到全局或 UseAnthropic。
	OCRProtocol                string `gorm:"type:text" json:"ocr_protocol,omitempty"`
	OCRExtraBody               string `gorm:"type:text" json:"ocr_extra_body,omitempty"`
	OCRLanguage                string `gorm:"type:text" json:"ocr_language,omitempty"`
	OCRBinary                  string `gorm:"type:text" json:"ocr_binary,omitempty"`
	OCRToolsPath               string `gorm:"type:text" json:"ocr_tools_path,omitempty"`
	OCRTelemetryEnabled        bool   `gorm:"not null;default:false" json:"ocr_telemetry_enabled"`
	OCRTelemetryExporter       string `gorm:"type:text" json:"ocr_telemetry_exporter,omitempty"`
	OCRTelemetryOTLPEndpoint   string `gorm:"type:text" json:"ocr_telemetry_otlp_endpoint,omitempty"`
	OCRTelemetryContentLogging bool   `gorm:"not null;default:false" json:"ocr_telemetry_content_logging"`
	// RulePackageIDs 项目引用的规则包 id 列表（在项目内联规则之外，额外引用规则库中的包）。
	RulePackageIDs datatypes.JSON `gorm:"type:jsonb" json:"rule_package_ids,omitempty"`
}

func (ProjectPolicy) TableName() string {
	return "project_policies"
}
