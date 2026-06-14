package models

import (
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// ReviewSession represents a review session that tracks the complete lifecycle of a review
type ReviewSession struct {
	gorm.Model      `json:"-"`
	SessionID       string         `gorm:"type:text;uniqueIndex;not null" json:"session_id"`
	ReviewID        string         `gorm:"type:text;not null;index" json:"review_id"`
	Round           int            `gorm:"not null;default:1" json:"round"`
	Phase           string         `gorm:"type:text;not null" json:"phase"`
	Status          string         `gorm:"type:text;not null;default:'active'" json:"status"`
	Progress        float64        `gorm:"not null;default:0" json:"progress"`
	AgentResults    datatypes.JSON `gorm:"type:jsonb" json:"agent_results,omitempty"`
	Discussion      datatypes.JSON `gorm:"type:jsonb" json:"discussion,omitempty"`
	FinalReport     datatypes.JSON `gorm:"type:jsonb" json:"final_report,omitempty"`
	Requirements    datatypes.JSON `gorm:"type:jsonb" json:"requirements,omitempty"`
	Metadata        datatypes.JSON `gorm:"type:jsonb" json:"metadata,omitempty"`
	CompletedAt     *time.Time     `json:"completed_at,omitempty"`
}

// TableName returns the table name for ReviewSession
func (ReviewSession) TableName() string {
	return "review_sessions"
}

// SessionPhase represents the phase of a review session
type SessionPhase string

const (
	SessionPhaseContext      SessionPhase = "context"
	SessionPhaseChangeContext SessionPhase = "change_context"
	SessionPhaseAnalysis     SessionPhase = "analysis"
	SessionPhaseReviews      SessionPhase = "reviews"
	SessionPhaseAggregation  SessionPhase = "aggregation"
	SessionPhaseDiscussion   SessionPhase = "discourse"
	SessionPhaseSynthesis    SessionPhase = "synthesis"
	SessionPhaseComplete     SessionPhase = "complete"
)

// SessionStatus represents the status of a review session
type SessionStatus string

const (
	SessionStatusActive    SessionStatus = "active"
	SessionStatusCompleted SessionStatus = "completed"
	SessionStatusFailed    SessionStatus = "failed"
	SessionStatusCancelled SessionStatus = "cancelled"
)

// AgentResultData represents the result of an agent in a session
type AgentResultData struct {
	AgentName  string        `json:"agent_name"`
	Findings   []FindingData `json:"findings"`
	Summary    string        `json:"summary"`
	Confidence float64       `json:"confidence"`
	Duration   int64         `json:"duration_ms"`
	TokensUsed int64         `json:"tokens_used"`
	Warnings   []WarningData `json:"warnings"`
}

// FindingData represents a finding in a session
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

// WarningData represents a warning in a session
type WarningData struct {
	Type    string `json:"type"`
	Message string `json:"message"`
	Path    string `json:"path"`
}

// DiscussionData represents the discussion in a session
type DiscussionData struct {
	ID        string              `json:"id"`
	Round     int                 `json:"round"`
	Entries   []DiscussionEntry   `json:"entries"`
	Summary   string              `json:"summary"`
	CreatedAt time.Time           `json:"created_at"`
}

// DiscussionEntry represents a single discussion entry
type DiscussionEntry struct {
	ID          string   `json:"id"`
	AgentName   string   `json:"agent_name"`
	FindingID   string   `json:"finding_id"`
	Response    string   `json:"response"`
	Content     string   `json:"content"`
	Evidence    string   `json:"evidence"`
	Confidence  float64  `json:"confidence"`
	References  []string `json:"references"`
	CreatedAt   time.Time `json:"created_at"`
}

// FinalReportData represents the final report of a session
type FinalReportData struct {
	Verdict          string              `json:"verdict"`
	Summary          string              `json:"summary"`
	Blockers         []FindingData       `json:"blockers"`
	ShouldFix        []FindingData       `json:"should_fix"`
	Suggestions      []FindingData       `json:"suggestions"`
	Requirements     *RequirementsData   `json:"requirements,omitempty"`
	ClarifyingQuestions []string         `json:"clarifying_questions,omitempty"`
	Score            int                 `json:"score"`
	GatePassed       bool                `json:"gate_passed"`
	GateReason       string              `json:"gate_reason"`
}

// RequirementsData represents requirements assessment
type RequirementsData struct {
	Inline    []string          `json:"inline"`
	Files     []string          `json:"files"`
	URLs      []string          `json:"urls"`
	Context   map[string]string `json:"context"`
	Coverage  []CoverageItem    `json:"coverage"`
	Summary   string            `json:"summary"`
}

// CoverageItem represents requirement coverage
type CoverageItem struct {
	Requirement string  `json:"requirement"`
	Status      string  `json:"status"` // met, partial, missing, unclear
	Confidence  float64 `json:"confidence"`
	Evidence    string  `json:"evidence"`
}

// SessionMetadata contains additional session metadata
type SessionMetadata struct {
	Branch      string    `json:"branch"`
	BaseSHA     string    `json:"base_sha"`
	HeadSHA     string    `json:"head_sha"`
	FilesChanged int      `json:"files_changed"`
	DiffSize    int       `json:"diff_size"`
	StartedAt   time.Time `json:"started_at"`
	CompletedAt time.Time `json:"completed_at,omitempty"`
	Duration    int64     `json:"duration_ms"`
}
