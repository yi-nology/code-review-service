package ocr

import "strings"

// 默认严重级别：未指定时使用 "major"。
const DefaultSeverity = "major"

// Config 是 open-code-review 的运行时配置。
//
// 阶段 A 设计：
//   - Config 由 settings (DB) + overrides (请求/项目策略) + fallback (YAML) 三层合并而来
//   - BuildConfig 是合并的单一实现（吸收 runner.BuildConfig + ocr.ConfigResolver）
//   - Validate 校验合并后的配置完整性
//   - AppendEnv 把 Config 转成 ocr 二进制的环境变量
//
// 此类型取代 runner.OpenCodeReviewConfig 和 domain.OCROverrides（合并语义）。
type Config struct {
	// 基础
	Binary          string
	Audience        string
	RulePath        string
	DefaultSeverity string
	Concurrency     int
	MaxGitProcs     int
	TimeoutMinutes  int
	MaxRetries      int

	// LLM
	URL          string
	Model        string
	AuthToken    string
	AuthHeader   string
	ExtraHeaders string
	UseAnthropic bool
	// Protocol 显式指定 LLM 协议（anthropic / openai / openai-responses）。
	// 留空时 OCR 按 UseAnthropic 推断（向后兼容）；非空时透传 OCR_LLM_PROTOCOL（OCR v1.7.10+），
	// 优先级高于 UseAnthropic。
	Protocol  string
	ExtraBody string

	// 行为
	Language       string
	Background     string
	BackgroundFile string // --background-file 路径（OCR v1.7.6+），与 Background 叠加

	// 工具
	MaxTools    int
	ToolsPath   string
	ToolsConfig string
	McpServers  string

	// 遥测
	TelemetryEnabled        bool
	TelemetryExporter       string
	TelemetryOTLPEndpoint   string
	TelemetryContentLogging bool

	// 模型参数
	ContextWindow   int
	MaxOutputTokens int

	// GitHubMirror 是 OCR 资产的 GitHub 下载镜像地址（OCR v1.9.5+）。
	GitHubMirror string
}

// Fallback 是 YAML 文件兜底配置，用于在没有 DB settings 时的最低保障。
type Fallback struct {
	Binary          string
	Audience        string
	RulePath        string
	DefaultSeverity string
	Concurrency     int
	MaxGitProcs     int
	TimeoutMinutes  int
}

// Defaults 返回合理默认的 Fallback（不读 DB / 不读 YAML）。
// 主要供测试与 fallback fallback 使用。
//
// Audience 默认 "agent"：review-service 是无人值守服务，调用方始终是程序而非人，
// 必须传 --audience agent 才能拿到纯净的 JSON 输出（OCR 的 human 模式会在 stdout
// 混进度 UI，污染 --format json 的可解析输出）。
func Defaults() Fallback {
	return Fallback{
		Binary:          "ocr",
		Audience:        "agent",
		DefaultSeverity: DefaultSeverity,
		Concurrency:     2,
		MaxGitProcs:     4,
		TimeoutMinutes:  10,
	}
}

// ToConfig 把 Fallback 转成 Config（覆盖基础字段，其它字段为零值）。
// 测试与简易启动路径用。
func (f Fallback) ToConfig() Config {
	return BuildConfig(&SettingsData{}, OCROverrides{}, f, nil)
}

// Validate 校验 Config 的完整性。
func (c Config) Validate() error {
	if c.Binary == "" {
		return errConfig("binary path is required")
	}
	if c.TimeoutMinutes > 0 && c.TimeoutMinutes > 120 {
		return errConfig("timeout_minutes exceeds maximum (120): %d", c.TimeoutMinutes)
	}
	if c.Concurrency > 0 && c.Concurrency > 32 {
		return errConfig("concurrency exceeds maximum (32): %d", c.Concurrency)
	}
	return nil
}

// BuildConfig 是配置合并的单一实现。
//
// 配置优先级（从高到低）：
//  1. overrides — 请求级 / 项目策略级覆盖
//  2. settings  — 数据库全局配置（SettingsData）
//  3. fallback  — YAML 文件兜底
//  4. 内置默认值（audience="agent"：服务端调用方始终是程序）
//
// 阶段 A：本函数是原 runner.BuildConfig 的对等实现（保持行为一致）。
// 阶段 A+1：把 settings/fallback 类型从 models.OCRSettings + OpenCodeReviewConfig
// 改为本包定义的 SettingsSource 接口（消除 5 个等价接口）。
func BuildConfig(settings *SettingsData, overrides OCROverrides, fallback Fallback, tools ToolSettings) Config {
	cfg := Config{
		Binary:          firstNonBlank(settings.Binary, fallback.Binary),
		Audience:        firstNonBlank(overrides.Audience, settings.Audience, fallback.Audience, "agent"),
		RulePath:        fallback.RulePath,
		DefaultSeverity: firstNonBlank(overrides.DefaultSeverity, settings.DefaultSeverity, fallback.DefaultSeverity, DefaultSeverity),
		URL:             firstNonBlank(overrides.URL, settings.URL),
		Model:           firstNonBlank(overrides.Model, settings.Model),
		AuthToken:       firstNonBlank(overrides.AuthToken, settings.AuthToken),
		AuthHeader:      firstNonBlank(overrides.AuthHeader, settings.AuthHeader),
		ExtraHeaders:    firstNonBlank(overrides.ExtraHeaders, settings.ExtraHeaders),
		UseAnthropic:    overrides.UseAnthropic || settings.UseAnthropic,
		Protocol:        firstNonBlank(overrides.Protocol, settings.Protocol),
		ExtraBody: ResolveExtraBody(
			firstNonBlank(overrides.ExtraBody, settings.ExtraBody),
			firstNonBlank(settings.ReasoningMode, "light"),
			settings.ReasoningBudget,
			settings.Provider,
		),
		Language:                firstNonBlank(overrides.Language, settings.Language),
		MaxTools:                firstPositive(overrides.MaxTools, settings.MaxTools),
		Background:              firstNonBlank(overrides.Background, settings.Background),
		BackgroundFile:          firstNonBlank(overrides.BackgroundFile, settings.BackgroundFile),
		ToolsPath:               firstNonBlank(overrides.ToolsPath, settings.ToolsPath),
		ToolsConfig:             settings.ToolsConfig,
		McpServers:              settings.McpServers,
		ContextWindow:           settings.ContextWindow,
		MaxOutputTokens:         settings.MaxOutputTokens,
		TelemetryEnabled:        settings.TelemetryEnabled,
		TelemetryExporter:       settings.TelemetryExporter,
		TelemetryOTLPEndpoint:   settings.TelemetryOTLPEndpoint,
		TelemetryContentLogging: settings.TelemetryContentLogging,
		GitHubMirror:            settings.GitHubMirror,
	}
	cfg.Concurrency = firstPositive(overrides.Concurrency, settings.Concurrency, fallback.Concurrency)
	cfg.MaxGitProcs = firstPositive(overrides.MaxGitProcs, settings.MaxGitProcs, fallback.MaxGitProcs)
	cfg.TimeoutMinutes = firstPositive(overrides.TimeoutMinutes, settings.TimeoutMinutes, fallback.TimeoutMinutes)

	if cfg.ToolsPath == "" && cfg.ToolsConfig == "" {
		cfg.ToolsConfig = defaultToolsConfig(tools, "diff")
	}
	return cfg
}

// DefaultToolsConfig 返回首个匹配 mode 且启用的工具包配置 JSON。
// tools 为 nil 或查询失败时返回空串。
func defaultToolsConfig(tools ToolSettings, mode string) string {
	if tools == nil {
		return ""
	}
	items, err := tools.List(backgroundCtx())
	if err != nil {
		return ""
	}
	for i := range items {
		if items[i].Enabled && (items[i].ScanType == "" || items[i].ScanType == "all" || items[i].ScanType == mode) {
			return items[i].Config
		}
	}
	return ""
}

// AppendEnv 把 Config 转成 ocr 二进制的环境变量。
//
// 这是单一真相，所有 OCR 命令调用都应使用此函数构造 env（吸收 runner.AppendLLMEnv）。
// hasExplicitLLMCredential 表示 OCR 已经通过 OCR_LLM_TOKEN 显式拿到了 LLM 凭证。
//
// 用于决定环境里的 provider 凭证（OPENAI_API_KEY 等）算不算「冗余」：
// 显式下发过凭证时，环境里的同类凭证只会扩大泄露面；没下发过时，它们
// 可能正是 OCR 赖以跑 LLM 的东西，摘掉会让评审跑不起来。
func (c Config) hasExplicitLLMCredential() bool {
	return strings.TrimSpace(c.AuthToken) != ""
}

func (c Config) AppendEnv(env []string) []string {
	if c.URL != "" {
		env = append(env, "OCR_LLM_URL="+c.URL)
	}
	if c.Model != "" {
		env = append(env, "OCR_LLM_MODEL="+c.Model)
	}
	if c.AuthToken != "" {
		env = append(env, "OCR_LLM_TOKEN="+c.AuthToken)
	}
	if c.AuthHeader != "" {
		env = append(env, "OCR_LLM_AUTH_HEADER="+c.AuthHeader)
	}
	if c.ExtraHeaders != "" {
		env = append(env, "OCR_LLM_EXTRA_HEADERS="+c.ExtraHeaders)
	}
	// OCR v1.7.10+ 引入 OCR_LLM_PROTOCOL（取值 anthropic/openai/openai-responses），
	// 优先级高于 OCR_USE_ANTHROPIC。显式 Protocol 非空时只设 PROTOCOL，避免与 UseAnthropic 冲突；
	// Protocol 留空时维持旧 bool 行为，兼容 v1.7.5 及更早 binary 与现存部署。
	if c.Protocol != "" {
		env = append(env, "OCR_LLM_PROTOCOL="+c.Protocol)
	} else if c.UseAnthropic {
		env = append(env, "OCR_USE_ANTHROPIC=true")
	} else {
		env = append(env, "OCR_USE_ANTHROPIC=false")
	}
	if c.ExtraBody != "" {
		env = append(env, "OCR_LLM_EXTRA_BODY="+c.ExtraBody)
	}
	if c.ContextWindow > 0 {
		env = append(env, formatIntEnv("OCR_CONTEXT_WINDOW", c.ContextWindow))
	}
	if c.MaxOutputTokens > 0 {
		env = append(env, formatIntEnv("OCR_MAX_OUTPUT_TOKENS", c.MaxOutputTokens))
	}
	if c.TelemetryEnabled {
		env = append(env, "OCR_ENABLE_TELEMETRY=true")
	}
	if c.TelemetryExporter != "" {
		env = append(env, "OCR_TELEMETRY_EXPORTER="+c.TelemetryExporter)
	}
	if c.TelemetryOTLPEndpoint != "" {
		env = append(env, "OCR_TELEMETRY_OTLP_ENDPOINT="+c.TelemetryOTLPEndpoint)
	}
	if c.TelemetryContentLogging {
		env = append(env, "OCR_CONTENT_LOGGING=true")
	}
	if c.GitHubMirror != "" {
		env = append(env, "OCR_GITHUB_MIRROR="+c.GitHubMirror)
	}
	return env
}

// 阶段 A+1：将 modelOrEmpty / languageOrEmpty 等 helper 改为 settings.* 方法，
// 取代外部类型 OCROverrides 的扩展。
