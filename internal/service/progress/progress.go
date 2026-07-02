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
			if message != "" {
				p.Phases[i].Description = message
			}
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
	// 并行审查阶段：用各 agent 的平均进度驱动该阶段进度条，
	// 避免阶段进度长时间卡 0%（仅 agent 进度在涨）。
	if p.Phase == PhaseReviews && len(p.AgentStatus) > 0 {
		agg := p.aggregateAgentProgress()
		p.Progress = agg
		p.setPhaseProgress(PhaseReviews, agg)
	}
	p.UpdatedAt = time.Now()
}

// aggregateAgentProgress 返回所有 agent 进度的算术平均值。
// 所有 agent 完成时为 100，否则 < 100（阶段完成回调负责最终置 100）。
func (p *ReviewProgress) aggregateAgentProgress() float64 {
	if len(p.AgentStatus) == 0 {
		return 0
	}
	var sum float64
	for _, a := range p.AgentStatus {
		sum += a.Progress
	}
	return sum / float64(len(p.AgentStatus))
}

// setPhaseProgress 更新指定阶段的进度数值（不改变其状态）。
func (p *ReviewProgress) setPhaseProgress(phase Phase, progress float64) {
	for i, ph := range p.Phases {
		if ph.Name == string(phase) {
			p.Phases[i].Progress = progress
			return
		}
	}
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

func (p *ReviewProgress) IsActive() bool {
	return p.Status != "completed" && p.Status != "failed" && p.Status != "cancelled"
}

func (p *ReviewProgress) Clone() *ReviewProgress {
	if p == nil {
		return nil
	}
	cp := *p
	if p.Phases != nil {
		cp.Phases = make([]PhaseInfo, len(p.Phases))
		copy(cp.Phases, p.Phases)
	}
	if p.AgentStatus != nil {
		cp.AgentStatus = make(map[string]AgentStatus, len(p.AgentStatus))
		for k, v := range p.AgentStatus {
			cp.AgentStatus[k] = v
		}
	}
	return &cp
}
