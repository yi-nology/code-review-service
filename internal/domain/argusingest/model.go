// Package argusingest 是 Argus（PR 自动审查 Bot）审查结果上报的领域类型。
//
// 与 feedback 包同范式：领域类型纯净（无 GORM 标签/持久化细节），
// postgres 实现在边界处做 domain ↔ model 映射。
// 契约（push 模型）：Argus 审查完成后 POST /api/v1/argus/reviews 上报，
// 幂等键 = (platform, owner, repo, number, head_sha)——同 PR 同 SHA 重审覆盖；
// 意见按 (review, fp) 唯一，fp 为 Argus 侧规范指纹（Finding.FP，v3.8.0 起
// Merger 写定、只读贯穿），与 finding_feedback 的指纹防重同源思路。
package argusingest

import "time"

// Verdict 词表（与 Argus R5 真值表对齐；error = Argus 侧审查失败但任务存在）。
const (
	VerdictApprove        = "approve"
	VerdictComment        = "comment"
	VerdictRequestChanges = "request_changes"
	VerdictError          = "error"
)

// Severity 词表（Argus 钉死三档）。
const (
	SeverityHigh   = "high"
	SeverityMedium = "medium"
	SeverityLow    = "low"
)

// Review Argus 一次审查任务的上报实体（任务维度）。
type Review struct {
	ID uint `json:"id"`
	// ArgusTaskID 是 Argus 侧任务 ID（溯源用；幂等以业务键为准，重审覆盖后为最新任务）。
	ArgusTaskID string `json:"argus_task_id"`
	Platform    string `json:"platform"`
	Owner       string `json:"owner"`
	Repo        string `json:"repo"`
	Number      string `json:"number"`
	Title       string `json:"title"`
	HeadSHA     string `json:"head_sha"`
	BaseBranch  string `json:"base_branch,omitempty"`
	// Verdict 取 Argus R5 真值表结果（非模型自评）。
	Verdict string `json:"verdict"`
	// 意见计数（落库时按 Findings 聚合冗余，列表页免 join）。
	FindingsTotal  int      `json:"findings_total"`
	FindingsHigh   int      `json:"findings_high"`
	FindingsMedium int      `json:"findings_medium"`
	FindingsLow    int      `json:"findings_low"`
	Plugins        []string `json:"plugins,omitempty"`
	DurationMS     int64    `json:"duration_ms,omitempty"`
	TokenUsage     int      `json:"token_usage,omitempty"`
	// ErrorMessage 仅 verdict=error 时有意义（argus 侧审查失败原因）。
	ErrorMessage string    `json:"error_message,omitempty"`
	OccurredAt   time.Time `json:"occurred_at"`
	Findings     []Finding `json:"findings,omitempty"`
}

// Finding Argus 单条审查意见。
type Finding struct {
	// FP 规范指纹（file+原始评语哈希；行号不入指纹）。空 = 上游旧数据。
	FP           string `json:"fp,omitempty"`
	File         string `json:"file"`
	Line         int    `json:"line"`
	Severity     string `json:"severity"`
	Comment      string `json:"comment"`
	SuggestedFix string `json:"suggested_fix,omitempty"`
	By           string `json:"by,omitempty"`
	// SuppressState 误报抑制回流状态：''（未处理）| 'suppressed'（argus 确认命中）。
	SuppressState string `json:"suppress_state,omitempty"`
}

// ListFilter 列表查询条件。
type ListFilter struct {
	// Repo 支持 repo 名或 owner/repo 组合匹配。
	Repo string
	// Platform 精确匹配（多平台同名 repo 区分用）。
	Platform string
	// Severity 非空时过滤"含该档意见"的任务（high → findings_high > 0）。
	Severity string
	Since    *time.Time
	Page     int
	PageSize int
}
