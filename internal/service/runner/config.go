package runner

import "strings"

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
	UseAnthropic            bool
	ExtraBody               string
	Language                string
	Background              string
	ToolsPath               string
	TelemetryEnabled        bool
	TelemetryExporter       string
	TelemetryOTLPEndpoint   string
	TelemetryContentLogging bool
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

func firstNonBlank(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func firstPositive(values ...int) int {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}
