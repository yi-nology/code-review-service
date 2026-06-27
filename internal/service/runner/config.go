package runner

import (
	"fmt"
	"strings"

	"github.com/oekit/pkg/slicex"
)

const (
	defaultBinary         = "ocr"
	defaultAudience       = "human"
	defaultSeverity       = "major"
	defaultConcurrency    = 2
	defaultMaxGitProcs    = 4
	defaultTimeoutMinutes = 10
)

type OpenCodeReviewConfig struct {
	Binary                  string
	Audience                string
	RulePath                string
	DefaultSeverity         string
	Concurrency             int
	MaxGitProcs             int
	TimeoutMinutes          int
	MaxRetries              int
	MaxTools                int
	URL                     string
	Model                   string
	AuthToken               string
	AuthHeader              string
	ExtraHeaders            string
	UseAnthropic            bool
	ExtraBody               string
	Language                string
	Background              string
	ToolsPath               string
	ToolsConfig             string
	TelemetryEnabled        bool
	TelemetryExporter       string
	TelemetryOTLPEndpoint   string
	TelemetryContentLogging bool
	ContextWindow           int // 上下文窗口大小（tokens），0=模型默认
	MaxOutputTokens         int // 最大输出 tokens，0=模型默认
}

// Validate 校验 OCR 配置的完整性。
func (c OpenCodeReviewConfig) Validate() error {
	if strings.TrimSpace(c.Binary) == "" {
		return fmt.Errorf("binary path is required")
	}
	if c.TimeoutMinutes > 0 && c.TimeoutMinutes > 120 {
		return fmt.Errorf("timeout_minutes exceeds maximum (120): %d", c.TimeoutMinutes)
	}
	if c.Concurrency > 0 && c.Concurrency > 32 {
		return fmt.Errorf("concurrency exceeds maximum (32): %d", c.Concurrency)
	}
	return nil
}

func NewOpenCodeReviewRunner(config OpenCodeReviewConfig) *OpenCodeReviewRunner {
	if strings.TrimSpace(config.Binary) == "" {
		config.Binary = defaultBinary
	}
	if strings.TrimSpace(config.Audience) == "" {
		config.Audience = defaultAudience
	}
	if strings.TrimSpace(config.DefaultSeverity) == "" {
		config.DefaultSeverity = defaultSeverity
	}
	if config.Concurrency <= 0 {
		config.Concurrency = defaultConcurrency
	}
	if config.MaxGitProcs <= 0 {
		config.MaxGitProcs = defaultMaxGitProcs
	}
	if config.TimeoutMinutes <= 0 {
		config.TimeoutMinutes = defaultTimeoutMinutes
	}
	return &OpenCodeReviewRunner{config: config}
}

// firstNonBlank 返回第一个非空白字符串。
func firstNonBlank(values ...string) string {
	return slicex.First(func(s string) bool { return strings.TrimSpace(s) != "" }, values...)
}

// firstPositive 返回第一个正数。
func firstPositive(values ...int) int {
	return slicex.First(func(v int) bool { return v > 0 }, values...)
}

// AppendLLMEnv 将 LLM 配置追加到环境变量列表
// 这是统一的环境变量构建函数，所有 OCR 命令调用都应使用此函数
func AppendLLMEnv(env []string, cfg OpenCodeReviewConfig) []string {
	if cfg.URL != "" {
		env = append(env, "OCR_LLM_URL="+cfg.URL)
	}
	if cfg.Model != "" {
		env = append(env, "OCR_LLM_MODEL="+cfg.Model)
	}
	if cfg.AuthToken != "" {
		env = append(env, "OCR_LLM_TOKEN="+cfg.AuthToken)
	}
	if cfg.AuthHeader != "" {
		env = append(env, "OCR_LLM_AUTH_HEADER="+cfg.AuthHeader)
	}
	if cfg.ExtraHeaders != "" {
		env = append(env, "OCR_LLM_EXTRA_HEADERS="+cfg.ExtraHeaders)
	}
	if cfg.UseAnthropic {
		env = append(env, "OCR_USE_ANTHROPIC=true")
	} else {
		env = append(env, "OCR_USE_ANTHROPIC=false")
	}
	if cfg.Language != "" {
		env = append(env, "OCR_LANGUAGE="+cfg.Language)
	}
	if cfg.ExtraBody != "" {
		env = append(env, "OCR_LLM_EXTRA_BODY="+cfg.ExtraBody)
	}
	if cfg.ContextWindow > 0 {
		env = append(env, fmt.Sprintf("OCR_CONTEXT_WINDOW=%d", cfg.ContextWindow))
	}
	if cfg.MaxOutputTokens > 0 {
		env = append(env, fmt.Sprintf("OCR_MAX_OUTPUT_TOKENS=%d", cfg.MaxOutputTokens))
	}
	if cfg.TelemetryEnabled {
		env = append(env, "OCR_ENABLE_TELEMETRY=true")
	}
	if cfg.TelemetryExporter != "" {
		env = append(env, "OCR_TELEMETRY_EXPORTER="+cfg.TelemetryExporter)
	}
	if cfg.TelemetryOTLPEndpoint != "" {
		env = append(env, "OCR_TELEMETRY_OTLP_ENDPOINT="+cfg.TelemetryOTLPEndpoint)
	}
	if cfg.TelemetryContentLogging {
		env = append(env, "OCR_CONTENT_LOGGING=true")
	}
	return env
}
