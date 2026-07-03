// Package session 是评审会话（review session）的领域类型（#6 迁移）。
// 包含 Session 聚合（对应 review_sessions 表）+ JSONB payload DTO（AgentResult/Discussion/
// FinalReport/Requirements 等，原位于 models 包，无 GORM 标签的纯 DTO）。
package session

import (
	"encoding/json"
	"time"
)

// Session 表示一次评审会话的完整生命周期记录。
// AgentResults/Discussion/FinalReport/Requirements/Metadata 为 JSONB，
// 领域层用 json.RawMessage 解耦 GORM datatypes.JSON。
type Session struct {
	SessionID    string          `json:"session_id"`
	ReviewID     string          `json:"review_id"`
	Round        int             `json:"round"`
	Phase        string          `json:"phase"`
	Status       string          `json:"status"`
	Progress     float64         `json:"progress"`
	AgentResults json.RawMessage `json:"agent_results,omitempty"`
	Discussion   json.RawMessage `json:"discussion,omitempty"`
	FinalReport  json.RawMessage `json:"final_report,omitempty"`
	Requirements json.RawMessage `json:"requirements,omitempty"`
	Metadata     json.RawMessage `json:"metadata,omitempty"`
	CompletedAt  *time.Time      `json:"completed_at,omitempty"`
	CreatedAt    time.Time       `json:"created_at"`
	UpdatedAt    time.Time       `json:"updated_at"`
}

// 状态枚举（与原 models.SessionStatus 常量值一致）。
const (
	StatusActive    = "active"
	StatusCompleted = "completed"
	StatusFailed    = "failed"
	StatusCancelled = "cancelled"
)

// 阶段枚举（与原 models.SessionPhase 常量值一致）。
const (
	PhaseContext       = "context"
	PhaseChangeContext = "change_context"
	PhaseAnalysis      = "analysis"
	PhaseReviews       = "reviews"
	PhaseAggregation   = "aggregation"
	PhaseDiscussion    = "discourse"
	PhaseSynthesis     = "synthesis"
	PhaseComplete      = "complete"
)

// AgentResultData 表示一个 agent 在会话中的结果。
type AgentResultData struct {
	AgentName  string        `json:"agent_name"`
	Findings   []FindingData `json:"findings"`
	Summary    string        `json:"summary"`
	Confidence float64       `json:"confidence"`
	Duration   int64         `json:"duration_ms"`
	TokensUsed int64         `json:"tokens_used"`
	Warnings   []WarningData `json:"warnings"`
}

// FindingData 表示会话中的一条 finding。
type FindingData struct {
	ID         string   `json:"id"`
	Severity   string   `json:"severity"`
	Category   string   `json:"category"`
	Path       string   `json:"path"`
	Line       int      `json:"line"`
	EndLine    int      `json:"end_line"`
	Title      string   `json:"title"`
	Body       string   `json:"body"`
	Suggestion string   `json:"suggestion"`
	Confidence float64  `json:"confidence"`
	Agents     []string `json:"agents"`
}

// WarningData 表示会话中的一条警告。
type WarningData struct {
	Type    string `json:"type"`
	Message string `json:"message"`
	Path    string `json:"path"`
}

// DiscussionData 表示会话中的讨论。
type DiscussionData struct {
	ID        string            `json:"id"`
	Round     int               `json:"round"`
	Entries   []DiscussionEntry `json:"entries"`
	Summary   string            `json:"summary"`
	CreatedAt time.Time         `json:"created_at"`
}

// DiscussionEntry 表示一条讨论条目。
type DiscussionEntry struct {
	ID         string    `json:"id"`
	AgentName  string    `json:"agent_name"`
	FindingID  string    `json:"finding_id"`
	Response   string    `json:"response"`
	Content    string    `json:"content"`
	Evidence   string    `json:"evidence"`
	Confidence float64   `json:"confidence"`
	References []string  `json:"references"`
	CreatedAt  time.Time `json:"created_at"`
}

// FinalReportData 表示会话的最终报告。
type FinalReportData struct {
	Verdict             string                   `json:"verdict"`
	Summary             string                   `json:"summary"`
	Blockers            []FindingData            `json:"blockers"`
	ShouldFix           []FindingData            `json:"should_fix"`
	Suggestions         []FindingData            `json:"suggestions"`
	Requirements        *RequirementsData        `json:"requirements,omitempty"`
	ClarifyingQuestions []ClarifyingQuestionData `json:"clarifying_questions,omitempty"`
	TechLeadAnalysis    *TechLeadAnalysisData    `json:"tech_lead_analysis,omitempty"`
	Score               int                      `json:"score"`
	GatePassed          bool                     `json:"gate_passed"`
	GateReason          string                   `json:"gate_reason"`
}

// ClarifyingQuestionData 表示一条需澄清的问题。
type ClarifyingQuestionData struct {
	Category string `json:"category"`
	Question string `json:"question"`
	Context  string `json:"context"`
}

// TechLeadAnalysisData 是 Tech Lead 分析的持久化产物。
type TechLeadAnalysisData struct {
	Summary         string   `json:"summary"`
	Risks           []string `json:"risks"`
	Recommendations []string `json:"recommendations"`
}

// RequirementsData 表示需求评估。
type RequirementsData struct {
	Inline   []string          `json:"inline"`
	Files    []string          `json:"files"`
	URLs     []string          `json:"urls"`
	Context  map[string]string `json:"context"`
	Coverage []CoverageItem    `json:"coverage"`
	Summary  string            `json:"summary"`
}

// CoverageItem 表示需求覆盖项。
type CoverageItem struct {
	Requirement string  `json:"requirement"`
	Status      string  `json:"status"`
	Confidence  float64 `json:"confidence"`
	Evidence    string  `json:"evidence"`
}

// SessionMetadata 包含会话附加元数据。
type SessionMetadata struct {
	Branch       string    `json:"branch"`
	BaseSHA      string    `json:"base_sha"`
	HeadSHA      string    `json:"head_sha"`
	FilesChanged int       `json:"files_changed"`
	DiffSize     int       `json:"diff_size"`
	StartedAt    time.Time `json:"started_at"`
	CompletedAt  time.Time `json:"completed_at,omitempty"`
	Duration     int64     `json:"duration_ms"`
}
