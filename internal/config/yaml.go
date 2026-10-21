package config

import (
	"github.com/oekit/registry/nacos"
	"github.com/yi-nology/code-review-service/internal/domain/notification"
)

// Config 配置结构
type Config struct {
	Port int `yaml:"port"`
	// ExternalBaseURL 服务对外基地址（如 http://code-review.me），用于把 PR 门禁
	// check 的 target_url 拼成可点击的评审详情页链接；留空则不携带 target_url
	// （GitLab 拒绝相对路径，整条 status 会写入失败）。
	ExternalBaseURL string             `yaml:"external_base_url"`
	Auth            AuthConfig         `yaml:"auth"`
	Database        DatabaseConfig     `yaml:"database"`
	Nacos           nacos.Bootstrap    `yaml:"nacos"`
	Tracing         TracingConfig      `yaml:"tracing"`
	Review          ReviewConfig       `yaml:"review"`
	Agents          AgentsConfig       `yaml:"agents"`
	RateLimit       RateLimitConfig    `yaml:"rate_limit"`
	Webhook         WebhookConfig      `yaml:"webhook"`
	Argus           ArgusConfig        `yaml:"argus"`
	Notification    NotificationConfig `yaml:"notification"`
	Cost            CostConfig         `yaml:"cost"`
	Memory          MemoryConfig       `yaml:"memory"`
}

// TracingConfig 链路追踪配置（上游 tracing.Config 的本地镜像）。
//
// 为什么不直接用 tracing.Config：上游该结构体**没有任何 yaml tag**，
// yaml.v3 的默认键名是「字段名整体小写」——`SampleRate` 的键是 `samplerate`，
// 不是常规的 `sample_rate`。于是配置里按惯例写的 `sample_rate` 永远匹配不上，
// 且因「未知键被静默忽略」而**无声失效**：prod 配的 `sample_rate: 0.1`
// 从未生效，实际跑的是 100% 采样。`service_name` 同理——它决定 OTel
// resource 的 service.name，不配就一直是空。
//
// 这里给一层带 tag 的镜像，把键名钉成常规 snake_case；
// 到上游结构的转换见 GetTracingConfig。
type TracingConfig struct {
	// ServiceName 写入 OTel resource 的 service.name。
	ServiceName string `yaml:"service_name"`
	// Endpoint OTLP 端点（grpc: host:port；http: 完整 URL 或基址）。
	Endpoint string `yaml:"endpoint"`
	// Insecure 跳过 TLS（本地开发用 true）。
	Insecure bool `yaml:"insecure"`
	// SampleRate 采样率 0.0~1.0；<=0 沿用上游语义（全量采样）。
	SampleRate float64 `yaml:"sample_rate"`
	// Protocol 导出协议：grpc（缺省）/ http。
	Protocol string `yaml:"protocol"`
	// Headers 随导出请求带上的附加头（认证等）。
	Headers map[string]string `yaml:"headers"`
}

// MemoryConfig 是跨会话长期记忆的配置（底座 agent-memory）。
//
// 记忆库是独立数据面：默认落 SQLite 文件，多实例部署须配 dsn 走 Postgres
// （SQLite 文件在多副本下各写各的，记忆会分裂）。
type MemoryConfig struct {
	// Enabled 是否启用长期记忆。关闭时评审行为与启用前一致。
	Enabled bool `yaml:"enabled"`
	// DSN Postgres 连接串；非空时走 Postgres。
	DSN string `yaml:"dsn"`
	// Path SQLite 文件路径（dsn 为空时生效）。
	Path string `yaml:"path"`
	// Namespace 记忆库命名空间前缀；底座按「前缀/平台/仓库」隔离。
	Namespace string `yaml:"namespace"`
	// RecallLimit 单次召回条数上限（0=用底座默认）。
	RecallLimit int `yaml:"recall_limit"`
	// RecallBudget 单次召回的 token 预算（0=用底座默认）。
	RecallBudget int `yaml:"recall_budget"`
	// RecallTimeout 单次召回的总时限（如 "2s"）。空/非法 = 用底座默认 2s。
	//
	// 召回在评审主链上：PG 挂起或慢查询时，没有时限就等于评审一直等下去。
	// 底座默认是 2s；要放开就显式配一个更大的值。
	RecallTimeout string `yaml:"recall_timeout"`
	// MaintenanceEnabled 是否启用记忆库后台维护（回收/合并/退役）。
	//
	// 记忆是长期资产，必须能被维护：不维护的话原始事件只进不出，
	// 同签名事实越堆越多，召回被重复内容稀释。
	// 关闭时记忆仍可正常捕获与召回，只是没有后台清理。
	MaintenanceEnabled bool `yaml:"maintenance_enabled"`
	// MaintenanceInterval 维护周期（如 "6h"）。空 = 用默认 6 小时。
	MaintenanceInterval string `yaml:"maintenance_interval"`
	// RawRetention 原始事件保留时长（如 "720h"）。空 = 用默认 30 天。
	//
	// 保留期越长，事实的证据链越完整（可回溯）；越短则库越小。
	RawRetention string `yaml:"raw_retention"`
}

// ArgusConfig Argus（PR 自动审查 Bot）对接：反馈回流（误报抑制代理）需要
// 服务端直连 argus，凭证不出服务端、不下发浏览器。
type ArgusConfig struct {
	// BotURL argus 服务基地址（如 http://argus:8899）；空 = 反馈回流关闭。
	BotURL string `yaml:"bot_url"`
	// BotToken argus 的 API token（即 argus 侧 ARGUS_APITOKEN）。
	BotToken string `yaml:"bot_token"`
}

// CostConfig 控制 LLM 成本与预算。
// MonthlyBudgetUSD 是全局月度成本预算上限（美元），0 表示不限制。
// 超额时 ReviewAction 决定行为：warn（仅告警，默认）/ block（拒绝新评审）。
type CostConfig struct {
	MonthlyBudgetUSD float64 `yaml:"monthly_budget_usd"`
	ReviewAction     string  `yaml:"review_action"` // "warn"（默认）或 "block"
}

// NotificationConfig 控制评审事件通知。
// Webhooks 配置外部推送渠道（飞书/企业微信/钉钉），空表示不推送外部。
// 站内通知默认开启（无需配置）。
type NotificationConfig struct {
	Webhooks []notification.WebhookChannelConfig `yaml:"webhooks"`
	// InAppDisabled 关闭站内通知（默认 false，即开启）。
	InAppDisabled bool `yaml:"inapp_disabled"`
}

// RateLimitConfig 控制全局 HTTP 限流（每秒请求数与突发桶大小）。
// 未配置时使用 server.go 中的内置默认值（100 rps / burst 200）。
type RateLimitConfig struct {
	RPS   int `yaml:"rps"`
	Burst int `yaml:"burst"`
}

type AuthConfig struct {
	Token string `yaml:"token"`
	// Insecure 显式允许空 Token 启动（仅 dev 环境）。
	// 生产环境（prod）下 Insecure=true 仍会启动失败。
	Insecure bool `yaml:"insecure"`
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

// WebhookConfig 控制 webhook 行为。
// BotUsername 用于识别「本服务自身 PR 评论」触发的回声事件，避免自循环。
type WebhookConfig struct {
	BotUsername string `yaml:"bot_username"`
}

// AgentsConfig 仅暴露实际被消费的键（workflow 开关与轮次）；
// agent 并发/超时在 internal/service/agent/manager.go 的 ManagerConfig 管理。
type AgentsConfig struct {
	Discussion   DiscussionConfig   `yaml:"discussion"`
	Requirements RequirementsConfig `yaml:"requirements"`
}

type DiscussionConfig struct {
	Enabled   bool `yaml:"enabled"`
	MaxRounds int  `yaml:"max_rounds"`
	MinAgents int  `yaml:"min_agents"`
}

type RequirementsConfig struct {
	Enabled bool `yaml:"enabled"`
}
