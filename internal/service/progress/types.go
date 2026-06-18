package progress

import "time"

type eventType string

const (
	eventPhaseStart    eventType = "phase_start"
	eventPhaseProgress eventType = "phase_progress"
	eventPhaseComplete eventType = "phase_complete"
	eventAgentStatus   eventType = "agent_status"
	eventError         eventType = "error"
	eventComplete      eventType = "complete"
)

type Phase string

const (
	PhaseContext       Phase = "context"
	PhaseChangeContext Phase = "change_context"
	PhaseAnalysis      Phase = "analysis"
	PhaseReviews       Phase = "reviews"
	PhaseAggregation   Phase = "aggregation"
	PhaseDiscussion    Phase = "discourse"
	PhaseSynthesis     Phase = "synthesis"
	PhaseComplete      Phase = "complete"
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
