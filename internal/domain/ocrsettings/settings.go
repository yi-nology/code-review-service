// Package ocrsettings 是 OCR 全局配置的领域类型（#6 迁移）。
package ocrsettings

import "encoding/json"

// Settings 是 OCR 引擎的全局配置（单例）。
// ExtraBody / ToolsConfig 为 JSONB，领域层用 json.RawMessage 解耦 GORM datatypes.JSON。
type Settings struct {
	Enabled         bool   `json:"enabled"`
	Binary          string `json:"binary,omitempty"`
	Audience        string `json:"audience,omitempty"`
	DefaultSeverity string `json:"default_severity,omitempty"`
	Provider        string `json:"provider,omitempty"`
	URL             string `json:"url"`
	AuthToken       string `json:"auth_token,omitempty"`
	AuthHeader      string `json:"auth_header,omitempty"`
	ExtraHeaders    string `json:"extra_headers,omitempty"`
	ModelName       string `json:"model"`
	UseAnthropic    bool   `json:"use_anthropic"`
	// Protocol 控制 LLM 协议：留空=由 UseAnthropic 推断（兼容）；
	// anthropic / openai / openai-responses（OCR v1.7.10+，透传 OCR_LLM_PROTOCOL）。
	Protocol string `json:"protocol,omitempty"`
	// BackgroundFile 是 --background-file 的文件路径（OCR v1.7.6+），与 Background 叠加。
	BackgroundFile          string          `json:"background_file,omitempty"`
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
	McpServers              json.RawMessage `json:"mcp_servers,omitempty"`
	TelemetryEnabled        bool            `json:"telemetry_enabled"`
	TelemetryExporter       string          `json:"telemetry_exporter,omitempty"`
	TelemetryOTLPEndpoint   string          `json:"telemetry_otlp_endpoint,omitempty"`
	TelemetryContentLogging bool            `json:"telemetry_content_logging"`
	ContextWindow           int64           `json:"context_window"`
	MaxOutputTokens         int64           `json:"max_output_tokens"`
	// 策略配置（原 policy.*，迁移到 DB）
	PolicyFailOn      string   `json:"policy_fail_on,omitempty"`
	PolicyMaxComments int      `json:"policy_max_comments,omitempty"`
	PolicySummaryOnly bool     `json:"policy_summary_only"`
	PolicyIgnorePaths []string `json:"policy_ignore_paths,omitempty"`
	// 写回配置
	MaxDiffBytes int64 `json:"max_diff_bytes,omitempty"`
	// GitHubMirror 是 OCR 资产（规则包、工具包等）的 GitHub 下载镜像地址（OCR v1.9.5+）。
	// 留空=直连 github.com；国内环境可设为 https://ghproxy.com 等加速镜像。
	GitHubMirror string `json:"github_mirror,omitempty"`
}
