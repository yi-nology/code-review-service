package progress

import (
	"fmt"
	"sync"
)

type ProgressTracker struct {
	broadcaster *broadcaster
	reviewID    string
	sessionID   string
	progress    *ReviewProgress
	mu          sync.RWMutex
}

func newProgressTracker(broadcaster *broadcaster, reviewID, sessionID string, round int) *ProgressTracker {
	return &ProgressTracker{
		broadcaster: broadcaster,
		reviewID:    reviewID,
		sessionID:   sessionID,
		progress:    newReviewProgress(reviewID, sessionID, round),
	}
}

func (pt *ProgressTracker) GetRoomID() string {
	return "review:" + pt.reviewID
}

func (pt *ProgressTracker) GetProgress() *ReviewProgress {
	pt.mu.RLock()
	defer pt.mu.RUnlock()
	return pt.progress.Clone()
}

func (pt *ProgressTracker) UpdatePhase(phase Phase, status string, progress float64, message string) {
	pt.mu.Lock()
	pt.progress.UpdatePhase(phase, status, progress, message)
	pt.mu.Unlock()

	event := progressEvent{
		Type: eventPhaseProgress, Phase: phase, Status: status, Progress: progress, Message: message,
	}
	switch status {
	case "active":
		event.Type = eventPhaseStart
	case "completed":
		event.Type = eventPhaseComplete
	}
	pt.broadcaster.BroadcastEvent(pt.GetRoomID(), event)
}

func (pt *ProgressTracker) UpdateAgentStatus(agentName, status string, progress float64, message string) {
	pt.mu.Lock()
	pt.progress.UpdateAgentStatus(agentName, status, progress, message)
	phase := pt.progress.Phase
	prog := pt.progress.Progress
	pt.mu.Unlock()

	pt.broadcaster.BroadcastEvent(pt.GetRoomID(), progressEvent{
		Type: eventAgentStatus, Phase: phase, Status: "active",
		Progress: prog, Message: message,
		AgentStatus: map[string]AgentStatus{
			agentName: {Name: agentName, Status: status, Progress: progress, Message: message},
		},
	})
}

func (pt *ProgressTracker) SetError(err string) {
	pt.mu.Lock()
	pt.progress.SetError(err)
	phase := pt.progress.Phase
	prog := pt.progress.Progress
	pt.mu.Unlock()

	pt.broadcaster.BroadcastEvent(pt.GetRoomID(), progressEvent{
		Type: eventError, Phase: phase, Status: "failed",
		Progress: prog, Error: err, Message: "Review failed: " + err,
	})
}

func (pt *ProgressTracker) Complete() {
	pt.mu.Lock()
	pt.progress.Complete()
	pt.mu.Unlock()

	pt.broadcaster.BroadcastEvent(pt.GetRoomID(), progressEvent{
		Type: eventComplete, Phase: PhaseComplete, Status: "completed",
		Progress: 100, Message: "Review completed successfully",
	})
}

func (pt *ProgressTracker) ReportProgress(message string, progress float64) {
	pt.mu.Lock()
	pt.progress.UpdatePhase(PhaseAnalysis, "active", progress, message)
	pt.mu.Unlock()

	pt.broadcaster.BroadcastEvent(pt.GetRoomID(), progressEvent{
		Type: eventPhaseProgress, Phase: PhaseAnalysis, Status: "active",
		Progress: progress, Message: message,
	})
}

func (pt *ProgressTracker) ReportFileProgress(filename string, current, total int) {
	var pct float64
	if total > 0 {
		pct = float64(current) / float64(total) * 100
	}
	pt.ReportProgress(fmt.Sprintf("Processing file %d/%d: %s", current, total, filename), pct)
}

func (pt *ProgressTracker) ReportAgentStatus(agentName, status, message string) {
	pt.UpdateAgentStatus(agentName, status, 0, message)
}
