package config

import (
	"github.com/oekit/bootstrap/nacos"
	"github.com/oekit/tracing"
)

// Config 配置结构
type Config struct {
	Port     int             `yaml:"port"`
	Auth     AuthConfig      `yaml:"auth"`
	Database DatabaseConfig  `yaml:"database"`
	Nacos    nacos.Bootstrap `yaml:"nacos"`
	Tracing  tracing.Config  `yaml:"tracing"`
	Review   ReviewConfig    `yaml:"review"`
	OCR      OCRConfig       `yaml:"ocr"`
	Policy   PolicyConfig    `yaml:"policy"`
	Git      GitConfig       `yaml:"git"`
	Agents   AgentsConfig    `yaml:"agents"`
}

type AuthConfig struct {
	Token string `yaml:"token"`
}

type DatabaseConfig struct {
	DSN          string `yaml:"dsn"`
	MaxIdleConns int    `yaml:"max_idle_conns"`
	MaxOpenConns int    `yaml:"max_open_conns"`
}

type ReviewConfig struct {
	WorkerConcurrency  int    `yaml:"worker_concurrency"`
	WorkerPollInterval string `yaml:"worker_poll_interval"`
	WorkerStaleAfter   string `yaml:"worker_stale_after"`
	MaxDiffBytes       int    `yaml:"max_diff_bytes"`
}

type OCRConfig struct {
	Enabled    bool   `yaml:"enabled"`
	Binary     string `yaml:"binary"`
	Rule       string `yaml:"rule"`
	Command    string `yaml:"command"`
	ConfigFile string `yaml:"config_file"`
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
	Enabled         bool              `yaml:"enabled"`
	MaxConcurrency  int               `yaml:"max_concurrency"`
	AgentTimeout    string            `yaml:"agent_timeout"`
	TotalTimeout    string            `yaml:"total_timeout"`
	DefaultTeam     []AgentTeamMember `yaml:"default_team"`
	Discussion      DiscussionConfig  `yaml:"discussion"`
	Requirements    RequirementsConfig `yaml:"requirements"`
	ReviewMap       ReviewMapConfig   `yaml:"review_map"`
}

type AgentTeamMember struct {
	Name      string   `yaml:"name"`
	Count     int      `yaml:"count"`
	Focus     []string `yaml:"focus"`
	Enabled   bool     `yaml:"enabled"`
	Priority  int      `yaml:"priority"`
	AutoDetect bool    `yaml:"auto_detect,omitempty"`
	Triggers  []string `yaml:"triggers,omitempty"`
}

type DiscussionConfig struct {
	Enabled   bool `yaml:"enabled"`
	MaxRounds int  `yaml:"max_rounds"`
	MinAgents int  `yaml:"min_agents"`
}

type RequirementsConfig struct {
	Enabled bool `yaml:"enabled"`
}

type ReviewMapConfig struct {
	Enabled  bool `yaml:"enabled"`
	MinFiles int  `yaml:"min_files"`
}
