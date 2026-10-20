package progress

import (
	"time"

	dsession "github.com/yi-nology/code-review-service/internal/domain/session"
)

type eventType string

const (
	eventPhaseStart    eventType = "phase_start"
	eventPhaseProgress eventType = "phase_progress"
	eventPhaseComplete eventType = "phase_complete"
	eventAgentStatus   eventType = "agent_status"
	eventError         eventType = "error"
	eventComplete      eventType = "complete"
)

// Phase 是评审流程的阶段名。
//
// 取值直接引用领域层的阶段枚举（单一事实源）——此前本包另持一份字面量副本，
// 与 internal/domain/session 并存，改一处忘另一处就会漂移。
type Phase string

const (
	PhaseContext       Phase = dsession.PhaseContext
	PhaseChangeContext Phase = dsession.PhaseChangeContext
	PhaseAnalysis      Phase = dsession.PhaseAnalysis
	PhaseReviews       Phase = dsession.PhaseReviews
	PhaseAggregation   Phase = dsession.PhaseAggregation
	PhaseDiscussion    Phase = dsession.PhaseDiscussion
	PhaseSynthesis     Phase = dsession.PhaseSynthesis
	PhaseComplete      Phase = dsession.PhaseComplete
)

type progressEvent struct {
	Type        eventType              `json:"type"`
	Phase       Phase                  `json:"phase"`
	Status      string                 `json:"status"`
	Progress    float64                `json:"progress"`
	Message     string                 `json:"message"`
	AgentStatus map[string]AgentStatus `json:"agent_status,omitempty"`
	Error       string                 `json:"error,omitempty"`
	Timestamp   time.Time              `json:"timestamp"`
}

type AgentStatus struct {
	Name     string  `json:"name"`
	Status   string  `json:"status"`
	Progress float64 `json:"progress"`
	Message  string  `json:"message"`
	Duration int64   `json:"duration_ms,omitempty"`
	Error    string  `json:"error,omitempty"`
}

type PhaseInfo struct {
	Name        string    `json:"name"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Status      string    `json:"status"`
	Progress    float64   `json:"progress"`
	StartTime   time.Time `json:"start_time,omitempty"`
	EndTime     time.Time `json:"end_time,omitempty"`
	Duration    int64     `json:"duration_ms,omitempty"`
}
