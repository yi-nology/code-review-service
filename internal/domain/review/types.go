// Package domain 定义评审领域的核心类型、接口与常量。
//
// 它是整个评审子系统的纯领域叶子包：仅依赖 scoring/types/repository（均为叶子），
// 不依赖任何带行为的服务包。review.Service、executor、runner、provider、writeback、
// workflow、handler 等消费方都应依赖本包的类型与接口，从而与 review.Service 解耦。
package domain

import (
	"encoding/json"
	"strings"
	"time"

	types "github.com/yi-nology/code-review-service/internal/domain/finding"
	"github.com/yi-nology/code-review-service/internal/domain/stats"
	"github.com/oekit/pkg/ptrx"
	"github.com/oekit/pkg/stringx"
)

type Request struct {
	ReviewID        string `json:"-"`
	Provider        string `json:"provider"`
	Repo            string `json:"repo"`
	ProjectID       string `json:"project_id,omitempty"`
	PullRequestID   string `json:"pull_request_id,omitempty"`
	MergeRequestIID string `json:"merge_request_iid,omitempty"`
	BaseSHA         string `json:"base_sha"`
	HeadSHA         string `json:"head_sha"`
	// CommitSHA 用于单 commit 评审模式（OCR v1.9.5+ --commit）。
	// 与 PullRequestID/MergeRequestIID 互斥；非空时 BaseSHA/HeadSHA 可省略。
	CommitSHA    string `json:"commit_sha,omitempty"`
	CallbackMode string `json:"callback_mode,omitempty"`
	Wait         bool   `json:"wait"`
	Author       string `json:"author,omitempty"`
	AuthorName   string `json:"author_name,omitempty"`
	AuthorAvatar string `json:"author_avatar,omitempty"`
	Sender       string `json:"sender,omitempty"`
	// Requirements 是需求/spec/验收标准，透传给 workflow 做需求感知评审。
	Requirements []string `json:"requirements,omitempty"`
	// EphemeralReviewers 是本次评审临时加入的 reviewer 描述，不入库。
	EphemeralReviewers []EphemeralReviewer `json:"ephemeral_reviewers,omitempty"`
	// Engine 审查引擎：''|EngineOCR（默认，OCR 多智能体子进程）|EngineArgus
	// （外部 Argus Bot，触发后异步执行、经 /api/v1/argus/reviews 回报）。
	// 未显式指定时按项目策略 review_engine 解析，仍空则 OCR。
	Engine string `json:"engine,omitempty"`
	// TaskKind engine=argus 时的 argus 任务类型：''（即 TaskKindCodeReview）|
	// TaskKindCodeReview | TaskKindRequirementCompliance | TaskKindVisualCheck。
	// OCR 引擎不支持任务类型（校验期拒绝）。
	TaskKind string `json:"task_kind,omitempty"`
	// GateMode PR 门禁强弱依赖：''（auto：OCR 强 / argus 弱）| 'strong' | 'weak'。
	// 创建评审时从项目策略解析落盘，PR 状态回写按 EffectiveGateMode 决定红绿。
	GateMode string `json:"gate_mode,omitempty"`
}

// argus 任务类型词表（与 argusclient TaskKind 枚举一一对应；存储用小写字符串，
// 触发时映射为整数）。
const (
	TaskKindCodeReview            = "code_review"
	TaskKindRequirementCompliance = "requirement_compliance"
	TaskKindVisualCheck           = "visual_check"
	TaskKindDefault               = TaskKindCodeReview
)

// NormalizeTaskKind 归一任务类型：空 → code_review（argus 缺省即代码评审）；
// 未知值原样返回（校验期报错）。
func NormalizeTaskKind(kind string) string {
	kind = strings.ToLower(strings.TrimSpace(kind))
	if kind == "" {
		return TaskKindDefault
	}
	return kind
}

// 审查引擎词表。
const (
	EngineOCR   = "ocr"
	EngineArgus = "argus"
)

// EffectiveEngine 把持久化请求里的 engine 归一为生效引擎：” 或未知值（历史 OCR
// 记录无该键）一律视为 ocr；engine 随 Request JSONB 持久化的真源见 create.go resolveEngine。
func EffectiveEngine(engine string) string {
	if strings.ToLower(strings.TrimSpace(engine)) == EngineArgus {
		return EngineArgus
	}
	return EngineOCR
}

// IsValidEngineFilter 报告 engine 过滤值是否合法：”（不过滤）、ocr、argus。
func IsValidEngineFilter(engine string) bool {
	switch strings.ToLower(strings.TrimSpace(engine)) {
	case "", EngineOCR, EngineArgus:
		return true
	default:
		return false
	}
}

// EphemeralReviewer 描述一个临时的、不入库的 reviewer。
// 在 executor 层映射为 agent.EphemeralSpec。
type EphemeralReviewer struct {
	Name       string `json:"name,omitempty"`
	Background string `json:"background"`
}

type Response struct {
	ReviewID string    `json:"review_id"`
	Status   string    `json:"status"`
	Summary  string    `json:"summary"`
	Gate     Gate      `json:"gate"`
	Report   Report    `json:"report"`
	Analysis Analysis  `json:"analysis"`
	History  []Event   `json:"history,omitempty"`
	Findings []Finding `json:"findings,omitempty"`
	Error    string    `json:"error,omitempty"`
	// Writeback 记录回写（PR/MR 评论发布）的结果。
	Writeback WritebackSummary `json:"writeback,omitempty"`
	// ArgusTaskID engine=argus 时派发的 argus 任务 ID（回报关联与失败轮询用）。
	ArgusTaskID string `json:"argus_task_id,omitempty"`
	// Judgment 是 OCR 结果二次综合判断（agentkit LLM）。
	// 包含误报判断、优先级排序、人可读总结、门禁建议。
	Judgment *Judgment `json:"judgment,omitempty"`
}

// Judgment 是 Judge 对 OCR 结果的二次综合判断。
type Judgment struct {
	// OverallAssessment 综合评估（人可读，1-3 段文字）
	OverallAssessment string `json:"overall_assessment"`
	// Confidence 对 OCR 结果的置信度（high/medium/low）
	Confidence string `json:"confidence"`
	// RecommendedGate 建议的门禁结果（pass/fail/conditional_pass）
	RecommendedGate string `json:"recommended_gate"`
	// GateReason 门禁建议的理由
	GateReason string `json:"gate_reason"`
	// PrioritizedFindings 按优先级排序的 findings，含误报标记
	PrioritizedFindings []FindingJudgment `json:"prioritized_findings,omitempty"`
	// SummaryForHuman 面向人的简洁总结
	SummaryForHuman string `json:"summary_for_human,omitempty"`
}

// FindingJudgment 是单个 finding 的判断。
type FindingJudgment struct {
	Path         string `json:"path"`
	Line         int    `json:"line,omitempty"`
	Title        string `json:"title"`
	Severity     string `json:"severity"`
	IsLikelyTrue bool   `json:"is_likely_true"`
	Priority     int    `json:"priority"`
	Reason       string `json:"reason"`
}

// WritebackSummary 描述回写到 git 平台的最终结果。
// Status: "ok" / "skipped" / "stale" / "failed" / "disabled"。
type WritebackSummary struct {
	Status      string `json:"status"`
	Published   int    `json:"published"`
	Failed      int    `json:"failed"`
	Skipped     bool   `json:"skipped"`
	Stale       bool   `json:"stale,omitempty"`
	ExpectedSHA string `json:"expected_sha,omitempty"`
	ActualSHA   string `json:"actual_sha,omitempty"`
	Error       string `json:"error,omitempty"`
}

// Writeback status 常量。
const (
	WritebackStatusDisabled = "disabled"
	WritebackStatusSkipped  = "skipped"
	WritebackStatusStale    = "stale"
	WritebackStatusOK       = "ok"
	WritebackStatusFailed   = "failed"
)

// StatusSuccessWithPublishFailed 是评审成功但评论回写失败时的 review 状态。
// 用于把 PR-01 修复的「writeback 失败被吞」问题暴露给上游消费者。
const StatusSuccessWithPublishFailed = "success_with_publish_failed"

// Event 是审查流程中的一个步骤事件。
// 每个步骤记录：做了什么（Stage）、怎么做的（How）、输入了什么（Input）、
// 产出了什么（Output）、耗时多久（Duration）、当前状态（Status）。
// 用户可以通过审查详情页的审计面板查看每一步的完整细节。
type Event struct {
	Stage     string         `json:"stage"`
	Status    string         `json:"status"`
	Message   string         `json:"message"`
	Details   map[string]any `json:"details,omitempty"`
	CreatedAt time.Time      `json:"created_at"`
	// How 描述这一步是怎么做的（方法/工具/策略），用于审计。
	How string `json:"how,omitempty"`
	// Input 是这一步的输入数据（debug 用，可能很大）。
	Input any `json:"input,omitempty"`
	// Output 是这一步的输出数据（debug 用，可能很大）。
	Output any `json:"output,omitempty"`
	// DurationMs 是这一步的耗时（毫秒）。
	DurationMs int64 `json:"duration_ms,omitempty"`
}

type Analysis struct {
	Overview        string   `json:"overview,omitempty"`
	RootCause       []string `json:"root_cause,omitempty"`
	Risks           []string `json:"risks,omitempty"`
	Recommendations []string `json:"recommendations,omitempty"`
}

type Gate struct {
	Status string `json:"status"`
	Passed *bool  `json:"passed"`
	Reason string `json:"reason,omitempty"`
}

const (
	GatePending = "pending"
	GatePassed  = "passed"
	GateFailed  = "failed"
	GateError   = "error"
)

func PendingGate() Gate {
	return Gate{Status: GatePending}
}

func CompletedGate(passed bool, reason string) Gate {
	status := GateFailed
	if passed {
		status = GatePassed
	}
	return Gate{Status: status, Passed: ptrx.Of(passed), Reason: reason}
}

func ErrorGate(reason string) Gate {
	return Gate{Status: GateError, Passed: ptrx.Of(false), Reason: reason}
}

// Record 是 Review 聚合根的领域表示（#6 keystone）。
// Request/Response 为强类型（非 JSONB blob），消除上层 marshal/unmarshal 往返；
// RawOCR/RawOCRInput 保留 json.RawMessage（Analysis 派生输入）。
// 标量字段（Provider/Repo/Status/...）对应 models.Review 的索引列，供仓储查询/统计。
type Record struct {
	ID                 string          `json:"id"`
	IdempotencyKey     string          `json:"idempotency_key,omitempty"`
	RequestFingerprint string          `json:"request_fingerprint,omitempty"`
	Provider           string          `json:"provider"`
	Repo               string          `json:"repo"`
	Status             string          `json:"status"`
	Author             string          `json:"author,omitempty"`
	AuthorName         string          `json:"author_name,omitempty"`
	AuthorAvatar       string          `json:"author_avatar,omitempty"`
	Sender             string          `json:"sender,omitempty"`
	PullRequestID      string          `json:"pull_request_id,omitempty"`
	MergeRequestIID    string          `json:"merge_request_iid,omitempty"`
	BaseSHA            string          `json:"base_sha,omitempty"`
	HeadSHA            string          `json:"head_sha,omitempty"`
	Score              int             `json:"score"`
	Grade              string          `json:"grade,omitempty"`
	Summary            string          `json:"summary,omitempty"`
	RetryCount         int             `json:"retry_count"`
	Request            Request         `json:"request"`
	Response           Response        `json:"response"`
	RawOCR             json.RawMessage `json:"raw_ocr,omitempty"`
	RawOCRInput        json.RawMessage `json:"raw_ocr_input,omitempty"`
	CreatedAt          time.Time       `json:"created_at"`
	UpdatedAt          time.Time       `json:"updated_at"`
}

// ---- 仓储查询/统计 DTO（原 models/types.go，GORM-free，#6 迁移至 domain）----

// ListOptions 是 Review 列表查询过滤条件。
type ListOptions struct {
	Status     string
	GateStatus string
	Provider   string
	Repo       string
	// Engine 按生效审查引擎过滤：''（不过滤）| EngineOCR | EngineArgus。
	Engine string
	// From/To 限定统计时间窗口（基于 created_at）；零值表示不限制。周报用它限定最近 7 天。
	From     time.Time
	To       time.Time
	Page     int
	PageSize int
	// SortBy/SortOrder 控制列表排序（仅支持白名单字段）；空值时默认 created_at DESC。
	SortBy    string
	SortOrder string
}

// AggregateStats 是 Review 聚合统计结果。
// 显式 snake_case JSON tag，与其余 review API 契约保持一致（如 ListStats）。
type AggregateStats struct {
	Running    int `json:"running"`
	GateFailed int `json:"gate_failed"`
	Findings   int `json:"findings"`

	TotalTokens      int64 `json:"total_tokens"`
	InputTokens      int64 `json:"input_tokens"`
	OutputTokens     int64 `json:"output_tokens"`
	CacheReadTokens  int64 `json:"cache_read_tokens"`
	CacheWriteTokens int64 `json:"cache_write_tokens"`
}

// ProjectStatsOptions 是项目级 OCR 统计查询条件（Review/Contributor 共享）。
// ProjectStatsOptions 是 review/domain 的别名，指向规范定义 internal/domain/stats。
// ReviewRepository 继续使用 domain.ProjectStatsOptions（同一类型，零转换成本）。
type ProjectStatsOptions = stats.ProjectStatsOptions

// ProjectOCRMetrics 是项目级 OCR 指标聚合结果。
type ProjectOCRMetrics struct {
	ReviewCount        int64
	TotalTokens        int64
	InputTokens        int64
	OutputTokens       int64
	CacheReadTokens    int64
	CacheWriteTokens   int64
	TotalReviewedFiles int64
	AvgTokens          float64
	AvgScore           float64
	AvgFindingsDensity float64
	GradeDistribution  map[string]int64
	ToolCalls          []ToolCallStat
}

// ProjectOCRTrendPoint 是项目级 OCR 趋势的一个时间点。
type ProjectOCRTrendPoint struct {
	Period          string
	ReviewCount     int64
	TotalTokens     int64
	AvgTokens       float64
	AvgElapsedMs    float64
	AvgScore        float64
	FindingsDensity float64
}

// ToolCallStat 是工具调用次数统计。
type ToolCallStat struct {
	Tool  string `json:"tool"`
	Count int64  `json:"count"`
}

// 严重级别常量（无类型字符串常量，可用于任意 string 上下文）。
const (
	SeverityInfo     = "info"
	SeverityMinor    = "minor"
	SeverityMajor    = "major"
	SeverityCritical = "critical"
)

func NormalizeSeverity(value string) string {
	return types.NormalizeSeverity(value)
}

func InferSeverity(content, fallback string) string {
	lower := strings.ToLower(content)
	switch {
	case stringx.ContainsAny(lower, "critical") || stringx.ContainsAny(content, "严重问题", "致命"):
		return SeverityCritical
	case stringx.ContainsAny(lower, "minor") || stringx.ContainsAny(content, "轻微问题"):
		return SeverityMinor
	case stringx.ContainsAny(lower, "info") || stringx.ContainsAny(content, "提示"):
		return SeverityInfo
	default:
		return NormalizeSeverity(fallback)
	}
}

// Finding 是 types.Finding 的类型别名，作为整个代码库的单一 Finding 模型。
type Finding = types.Finding
