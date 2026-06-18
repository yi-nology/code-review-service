package progress

import "time"

type ReviewProgress struct {
	ReviewID    string                 `json:"review_id"`
	SessionID   string                 `json:"session_id"`
	Round       int                    `json:"round"`
	Phase       Phase                  `json:"phase"`
	Status      string                 `json:"status"`
	Progress    float64                `json:"progress"`
	Phases      []PhaseInfo            `json:"phases"`
	AgentStatus map[string]AgentStatus `json:"agent_status,omitempty"`
	StartTime   time.Time              `json:"start_time"`
	UpdatedAt   time.Time              `json:"updated_at"`
	Error       string                 `json:"error,omitempty"`
}

func newReviewProgress(reviewID, sessionID string, round int) *ReviewProgress {
	return &ReviewProgress{
		ReviewID:  reviewID,
		SessionID: sessionID,
		Round:     round,
		Phase:     PhaseContext,
		Status:    "starting",
		Progress:  0,
		Phases: []PhaseInfo{
			{Name: "context", Title: "Context Discovery", Description: "Loading configuration and discovering project standards", Status: "pending"},
			{Name: "change_context", Title: "Change Analysis", Description: "Analyzing git diff and understanding changes", Status: "pending"},
			{Name: "analysis", Title: "Tech Lead Analysis", Description: "Summarizing changes and identifying risks", Status: "pending"},
			{Name: "reviews", Title: "Parallel Reviews", Description: "Running multiple review agents in parallel", Status: "pending"},
			{Name: "aggregation", Title: "Aggregation", Description: "Merging findings from multiple agents", Status: "pending"},
			{Name: "discourse", Title: "Discourse", Description: "Agents debating and validating findings", Status: "pending"},
			{Name: "synthesis", Title: "Synthesis", Description: "Producing final prioritized review", Status: "pending"},
			{Name: "complete", Title: "Complete", Description: "Review completed", Status: "pending"},
		},
		AgentStatus: make(map[string]AgentStatus),
		StartTime:   time.Now(),
		UpdatedAt:   time.Now(),
	}
}

func (p *ReviewProgress) UpdatePhase(phase Phase, status string, progress float64, message string) {
	p.Phase = phase
	p.Status = status
	p.Progress = progress
	p.UpdatedAt = time.Now()

	for i, phaseInfo := range p.Phases {
		if phaseInfo.Name == string(phase) {
			p.Phases[i].Status = status
			p.Phases[i].Progress = progress
			if status == "active" && phaseInfo.StartTime.IsZero() {
				p.Phases[i].StartTime = time.Now()
			}
			if status == "completed" {
				p.Phases[i].EndTime = time.Now()
				if !phaseInfo.StartTime.IsZero() {
					p.Phases[i].Duration = time.Since(phaseInfo.StartTime).Milliseconds()
				}
			}
			break
		}
	}

	found := false
	for i, phaseInfo := range p.Phases {
		if phaseInfo.Name == string(phase) {
			found = true
			continue
		}
		if !found && phaseInfo.Status != "completed" {
			p.Phases[i].Status = "completed"
			p.Phases[i].Progress = 100
			if phaseInfo.StartTime.IsZero() {
				p.Phases[i].StartTime = time.Now()
			}
			p.Phases[i].EndTime = time.Now()
			if !phaseInfo.StartTime.IsZero() {
				p.Phases[i].Duration = time.Since(phaseInfo.StartTime).Milliseconds()
			}
		}
	}
}

func (p *ReviewProgress) UpdateAgentStatus(agentName, status string, progress float64, message string) {
	if p.AgentStatus == nil {
		p.AgentStatus = make(map[string]AgentStatus)
	}
	p.AgentStatus[agentName] = AgentStatus{
		Name: agentName, Status: status, Progress: progress, Message: message,
	}
	p.UpdatedAt = time.Now()
}

func (p *ReviewProgress) SetError(err string) {
	p.Error = err
	p.Status = "failed"
	p.UpdatedAt = time.Now()
}

func (p *ReviewProgress) Complete() {
	p.Phase = PhaseComplete
	p.Status = "completed"
	p.Progress = 100
	p.UpdatedAt = time.Now()
	for i := range p.Phases {
		if p.Phases[i].Status != "completed" {
			p.Phases[i].Status = "completed"
			p.Phases[i].Progress = 100
			p.Phases[i].EndTime = time.Now()
		}
	}
}

func (p *ReviewProgress) GetOverallProgress() float64 {
	if len(p.Phases) == 0 {
		return 0
	}
	total := 0.0
	for _, phase := range p.Phases {
		total += phase.Progress
	}
	return total / float64(len(p.Phases))
}

func (p *ReviewProgress) GetPhaseInfo(phase Phase) *PhaseInfo {
	for _, phaseInfo := range p.Phases {
		if phaseInfo.Name == string(phase) {
			return &phaseInfo
		}
	}
	return nil
}

func (p *ReviewProgress) IsActive() bool {
	return p.Status != "completed" && p.Status != "failed" && p.Status != "cancelled"
}

func (p *ReviewProgress) GetDuration() time.Duration {
	return time.Since(p.StartTime)
}
