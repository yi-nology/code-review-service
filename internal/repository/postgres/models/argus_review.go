package models

import "time"

// ArgusReview Argus 上报的审查任务（biz 键唯一：同 PR 同 SHA 重审覆盖）。
type ArgusReview struct {
	ID          uint   `gorm:"primarykey" json:"-"`
	ArgusTaskID string `gorm:"type:text;not null;index" json:"argus_task_id"`
	Platform    string `gorm:"type:text;not null;uniqueIndex:idx_argus_review_biz" json:"platform"`
	Owner       string `gorm:"type:text;not null;uniqueIndex:idx_argus_review_biz" json:"owner"`
	Repo        string `gorm:"type:text;not null;uniqueIndex:idx_argus_review_biz" json:"repo"`
	Number      string `gorm:"type:text;not null;uniqueIndex:idx_argus_review_biz" json:"number"`
	HeadSHA     string `gorm:"type:text;not null;uniqueIndex:idx_argus_review_biz" json:"head_sha"`
	Title       string `gorm:"type:text" json:"title"`
	BaseBranch  string `gorm:"type:text" json:"base_branch,omitempty"`
	Verdict     string `gorm:"type:text;not null;index" json:"verdict"`
	// 意见计数冗余（列表页免 join）。
	FindingsTotal  int `json:"findings_total"`
	FindingsHigh   int `json:"findings_high"`
	FindingsMedium int `json:"findings_medium"`
	FindingsLow    int `json:"findings_low"`
	// Plugins 参与插件名列表（JSON 数组文本）。
	Plugins    string `gorm:"type:text" json:"plugins,omitempty"`
	DurationMS int64  `json:"duration_ms,omitempty"`
	TokenUsage int    `json:"token_usage,omitempty"`
	// ErrorMessage 仅 verdict=error 时有意义（argus 侧审查失败原因）。
	ErrorMessage string    `gorm:"type:text;not null;default:''" json:"error_message,omitempty"`
	OccurredAt   time.Time `gorm:"index:idx_argus_reviews_occurred" json:"occurred_at"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// ArgusFinding Argus 单条审查意见。
// (review_id, fp) 唯一仅约束非空 fp（部分唯一索引，见 db.go 与 migration 0008）——
// 空 fp 的旧行允许多条共存；GORM 标签只建普通组合索引。
type ArgusFinding struct {
	ID       uint `gorm:"primarykey" json:"-"`
	ReviewID uint `gorm:"not null;index:idx_argus_finding_review" json:"review_id"`
	// FP Argus 侧规范指纹（可空 = 上游旧数据，多条共存合法）。
	FP           string `gorm:"type:text;not null;default:'';index:idx_argus_finding_review" json:"fp,omitempty"`
	File         string `gorm:"type:text;not null" json:"file"`
	Line         int    `json:"line"`
	Severity     string `gorm:"type:text;not null" json:"severity"`
	Comment      string `gorm:"type:text" json:"comment"`
	SuggestedFix string `gorm:"type:text" json:"suggested_fix,omitempty"`
	By           string `gorm:"type:text" json:"by,omitempty"`
	// SuppressState 误报抑制回流状态：''（未处理）| 'suppressed'（argus 确认命中）。
	SuppressState string    `gorm:"type:text;not null;default:''" json:"suppress_state,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
}
