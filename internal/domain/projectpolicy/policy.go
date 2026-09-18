// Package projectpolicy 是项目级评审策略的领域类型（#6 迁移）。
// 命名为 projectpolicy 以区别于 service/policy（策略引擎）。
package projectpolicy

import "encoding/json"

// DefaultFailOn 是自动创建（webhook 回调审查但项目尚未配置策略）时的默认门禁级别。
// 与前端新建表单默认值一致；选 major 是因为「阻断高风险及以上」对大多数业务仓库更合理。
const DefaultFailOn = "major"

// PR 门禁强弱依赖模式（gate_mode）取值。强弱语义的执行定义见
// review/domain.EffectiveGateMode：strong 终态失败写 failed/error（可配分支保护
// 必需检查）；weak 任何终态一律 success 留痕、不阻塞「所有检查通过后合并」类自动合并。
const (
	GateModeAuto   = ""       // 跟随引擎默认：OCR 强 / argus 弱
	GateModeStrong = "strong" // 强依赖（事实门禁）
	GateModeWeak   = "weak"   // 弱依赖（仅留痕）
)

// NewDefault 构造一条默认项目策略（仅 provider/repo + FailOn=DefaultFailOn，其余零值即「继承全局引擎配置」）。
// 用于审查执行时为未配置策略的项目自动落库，使其出现在项目策略页可被统一治理。
func NewDefault(provider, repo string) ProjectPolicy {
	return ProjectPolicy{Provider: provider, Repo: repo, FailOn: DefaultFailOn}
}

// ProjectPolicy 是某项目（provider+repo）的评审策略与 OCR 覆盖配置。
// IgnorePaths/Rules 为 JSONB，领域层用 json.RawMessage 解耦 GORM datatypes.JSON。
type ProjectPolicy struct {
	Provider    string          `json:"provider"`
	Repo        string          `json:"repo"`
	FailOn      string          `json:"fail_on"`
	IgnorePaths json.RawMessage `json:"ignore_paths"`
	Rules       json.RawMessage `json:"rules"`
	// ReviewEngine 该项目的默认审查引擎：''（默认 OCR）| 'ocr' | 'argus'。
	// webhook 创建评审时按此路由；请求显式指定 engine 时优先于本字段。
	ReviewEngine string `json:"review_engine,omitempty"`
	// GateMode PR 门禁强弱依赖模式：''（auto：OCR 强 / argus 弱）| 'strong' | 'weak'。
	GateMode           string `json:"gate_mode,omitempty"`
	OCREnabled         *bool  `json:"ocr_enabled,omitempty"`
	OCRConcurrency     int    `json:"ocr_concurrency,omitempty"`
	OCRTimeoutMinutes  int    `json:"ocr_timeout_minutes,omitempty"`
	OCRMaxTools        int    `json:"ocr_max_tools,omitempty"`
	OCRDefaultSeverity string `json:"ocr_default_severity,omitempty"`
	OCRBackground      string `json:"ocr_background,omitempty"`
	// OCRBackgroundFile 项目级 --background-file 路径覆盖（OCR v1.7.6+）；留空=回退到全局。
	OCRBackgroundFile string `json:"ocr_background_file,omitempty"`
	OCRAudience       string `json:"ocr_audience,omitempty"`
	OCRProvider       string `json:"ocr_provider,omitempty"`
	OCRURL            string `json:"ocr_url,omitempty"`
	OCRModel          string `json:"ocr_model,omitempty"`
	OCRMaxGitProcs    int    `json:"ocr_max_git_procs,omitempty"`
	OCRUseAnthropic   bool   `json:"ocr_use_anthropic"`
	// OCRProtocol 项目级 OCR 协议覆盖；留空=回退到全局（兼容旧策略）。
	OCRProtocol                string `json:"ocr_protocol,omitempty"`
	OCRExtraBody               string `json:"ocr_extra_body,omitempty"`
	OCRLanguage                string `json:"ocr_language,omitempty"`
	OCRBinary                  string `json:"ocr_binary,omitempty"`
	OCRToolsPath               string `json:"ocr_tools_path,omitempty"`
	OCRTelemetryEnabled        bool   `json:"ocr_telemetry_enabled"`
	OCRTelemetryExporter       string `json:"ocr_telemetry_exporter,omitempty"`
	OCRTelemetryOTLPEndpoint   string `json:"ocr_telemetry_otlp_endpoint,omitempty"`
	OCRTelemetryContentLogging bool   `json:"ocr_telemetry_content_logging"`
	// RulePackageIDs 项目引用的规则包 id 列表。
	RulePackageIDs json.RawMessage `json:"rule_package_ids,omitempty"`
}
