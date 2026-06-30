package config

import (
	"github.com/oekit/bootstrap/nacos"
	"github.com/oekit/tracing"
)

// Config 配置结构
type Config struct {
	Port      int             `yaml:"port"`
	Auth      AuthConfig      `yaml:"auth"`
	Database  DatabaseConfig  `yaml:"database"`
	Nacos     nacos.Bootstrap `yaml:"nacos"`
	Tracing   tracing.Config  `yaml:"tracing"`
	Review    ReviewConfig    `yaml:"review"`
	OCR       OCRConfig       `yaml:"ocr"`
	Policy    PolicyConfig    `yaml:"policy"`
	Git       GitConfig       `yaml:"git"`
	Agents    AgentsConfig    `yaml:"agents"`
	RateLimit RateLimitConfig `yaml:"rate_limit"`
	Webhook   WebhookConfig   `yaml:"webhook"`
}

// RateLimitConfig 控制全局 HTTP 限流（每秒请求数与突发桶大小）。
// 未配置时使用 server.go 中的内置默认值（100 rps / burst 200）。
type RateLimitConfig struct {
	RPS   int `yaml:"rps"`
	Burst int `yaml:"burst"`
}

type AuthConfig struct {
	Token string `yaml:"token"`
}

type DatabaseConfig struct {
	DSN          string `yaml:"dsn"`
	MaxIdleConns int    `yaml:"max_idle_conns"`
	MaxOpenConns int    `yaml:"max_open_conns"`
	AutoMigrate  *bool  `yaml:"auto_migrate,omitempty"`
}

type ReviewConfig struct {
	WorkerConcurrency  int           `yaml:"worker_concurrency"`
	WorkerPollInterval string        `yaml:"worker_poll_interval"`
	WorkerStaleAfter   string        `yaml:"worker_stale_after"`
	MaxDiffBytes       int           `yaml:"max_diff_bytes"`
	WorkspaceCleanup   CleanupConfig `yaml:"workspace_cleanup"`
}

type CleanupConfig struct {
	Enabled  bool   `yaml:"enabled"`
	MaxAge   string `yaml:"max_age"`
	Interval string `yaml:"interval"`
}

type OCRConfig struct {
	Enabled         bool   `yaml:"enabled"`
	Binary          string `yaml:"binary"`
	Rule            string `yaml:"rule"`
	Command         string `yaml:"command"`
	ConfigFile      string `yaml:"config_file"`
	ContextWindow   int    `yaml:"context_window"`
	MaxOutputTokens int    `yaml:"max_output_tokens"`
	// UpgradeInstallDir 是 ocr 在线升级时，当原二进制路径无写权限的回退安装目录。
	UpgradeInstallDir string `yaml:"upgrade_install_dir"`
	// GitHubRepo 指定 ocr 二进制的 GitHub 仓库（owner/repo），默认 alibaba/open-code-review。
	GitHubRepo string `yaml:"github_repo"`
}

// WebhookConfig 控制 webhook 行为。
// BotUsername 用于识别「本服务自身 PR 评论」触发的回声事件，避免自循环。
type WebhookConfig struct {
	BotUsername string `yaml:"bot_username"`
}

type PolicyConfig struct {
	FailOn      string   `yaml:"fail_on"`
	MaxComments int      `yaml:"max_comments"`
	SummaryOnly bool     `yaml:"summary_only"`
	IgnorePaths []string `yaml:"ignore_paths"`
}

type GitConfig struct {
	Platforms map[string]GitPlatformConfig `yaml:"platforms"`
}

type GitPlatformConfig struct {
	Enabled bool   `yaml:"enabled"`
	BaseURL string `yaml:"base_url"`
	Token   string `yaml:"token"`
}

type AgentsConfig struct {
	Enabled        bool               `yaml:"enabled"`
	MaxConcurrency int                `yaml:"max_concurrency"`
	AgentTimeout   string             `yaml:"agent_timeout"`
	TotalTimeout   string             `yaml:"total_timeout"`
	DefaultTeam    []AgentTeamMember  `yaml:"default_team"`
	Discussion     DiscussionConfig   `yaml:"discussion"`
	Requirements   RequirementsConfig `yaml:"requirements"`
}

type AgentTeamMember struct {
	Name       string   `yaml:"name"`
	Count      int      `yaml:"count"`
	Focus      []string `yaml:"focus"`
	Enabled    bool     `yaml:"enabled"`
	Priority   int      `yaml:"priority"`
	AutoDetect bool     `yaml:"auto_detect,omitempty"`
	Triggers   []string `yaml:"triggers,omitempty"`
}

type DiscussionConfig struct {
	Enabled   bool `yaml:"enabled"`
	MaxRounds int  `yaml:"max_rounds"`
	MinAgents int  `yaml:"min_agents"`
}

type RequirementsConfig struct {
	Enabled bool `yaml:"enabled"`
}
