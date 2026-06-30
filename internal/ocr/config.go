package ocr

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
	URL         string
	Model       string
	AuthToken   string
	AuthHeader  string
	ExtraHeaders string
	UseAnthropic bool
	ExtraBody    string

	// 行为
	Language   string
	Background string

	// 工具
	MaxTools    int
	ToolsPath   string
	ToolsConfig string

	// 遥测
	TelemetryEnabled        bool
	TelemetryExporter       string
	TelemetryOTLPEndpoint   string
	TelemetryContentLogging bool

	// 模型参数
	ContextWindow   int
	MaxOutputTokens int
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
func Defaults() Fallback {
	return Fallback{
		Binary:          "ocr",
		Audience:        "human",
		DefaultSeverity: "major",
		Concurrency:     2,
		MaxGitProcs:     4,
		TimeoutMinutes:  10,
	}
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
//  4. 内置默认值（如 audience="human"）
//
// 阶段 A：本函数是原 runner.BuildConfig 的对等实现（保持行为一致）。
// 阶段 A+1：把 settings/fallback 类型从 models.OCRSettings + OpenCodeReviewConfig
// 改为本包定义的 SettingsSource 接口（消除 5 个等价接口）。
func BuildConfig(settings *SettingsData, overrides OCROverrides, fallback Fallback, tools ToolSettings) Config {
	cfg := Config{
		Binary:          firstNonBlank(settings.Binary, fallback.Binary),
		Audience:        firstNonBlank(overrides.Audience, settings.Audience, fallback.Audience, "human"),
		RulePath:        fallback.RulePath,
		DefaultSeverity: firstNonBlank(overrides.DefaultSeverity, settings.DefaultSeverity, fallback.DefaultSeverity, "major"),
		URL:             firstNonBlank(overrides.URL, settings.URL),
		Model:           firstNonBlank(overrides.Model, settings.Model),
		AuthToken:       firstNonBlank(overrides.AuthToken, settings.AuthToken),
		AuthHeader:      firstNonBlank(overrides.AuthHeader, settings.AuthHeader),
		ExtraHeaders:    firstNonBlank(overrides.ExtraHeaders, settings.ExtraHeaders),
		UseAnthropic:    overrides.UseAnthropic || settings.UseAnthropic,
		ExtraBody: ResolveExtraBody(
			firstNonBlank(overrides.ExtraBody, settings.ExtraBody),
			firstNonBlank(settings.ReasoningMode, "light"),
			settings.ReasoningBudget,
			settings.Provider,
		),
		Language:        settings.Language,
		MaxTools:        firstPositive(overrides.MaxTools, settings.MaxTools),
		Background:      firstNonBlank(overrides.Background, settings.Background),
		ToolsPath:       firstNonBlank(overrides.ToolsPath, settings.ToolsPath),
		ToolsConfig:     settings.ToolsConfig,
		ContextWindow:   settings.ContextWindow,
		MaxOutputTokens: settings.MaxOutputTokens,
		TelemetryEnabled:        settings.TelemetryEnabled,
		TelemetryExporter:       settings.TelemetryExporter,
		TelemetryOTLPEndpoint:   settings.TelemetryOTLPEndpoint,
		TelemetryContentLogging: settings.TelemetryContentLogging,
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
	if c.UseAnthropic {
		env = append(env, "OCR_USE_ANTHROPIC=true")
	} else {
		env = append(env, "OCR_USE_ANTHROPIC=false")
	}
	if c.Language != "" {
		env = append(env, "OCR_LANGUAGE="+c.Language)
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
	return env
}

// 阶段 A+1：将 modelOrEmpty / languageOrEmpty 等 helper 改为 settings.* 方法，
// 取代外部类型 OCROverrides 的扩展。