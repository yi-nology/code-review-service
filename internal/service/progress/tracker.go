package progress

import (
	"fmt"
	"sync"
)

type ProgressTracker struct {
	broadcaster *Broadcaster
	reviewID    string
	sessionID   string
	progress    *ReviewProgress
	mu          sync.RWMutex
}

func NewProgressTracker(broadcaster *Broadcaster, reviewID, sessionID string, round int) *ProgressTracker {
	return &ProgressTracker{
		broadcaster: broadcaster,
		reviewID:    reviewID,
		sessionID:   sessionID,
		progress:    NewReviewProgress(reviewID, sessionID, round),
	}
}

func (pt *ProgressTracker) GetRoomID() string {
	return "review:" + pt.reviewID
}

func (pt *ProgressTracker) GetProgress() *ReviewProgress {
	pt.mu.RLock()
	defer pt.mu.RUnlock()
	return pt.progress
}

func (pt *ProgressTracker) UpdatePhase(phase Phase, status string, progress float64, message string) {
	pt.mu.Lock()
	pt.progress.UpdatePhase(phase, status, progress, message)
	pt.mu.Unlock()

	event := ProgressEvent{
		Type: EventPhaseProgress, Phase: phase, Status: status, Progress: progress, Message: message,
	}
	if status == "active" {
		event.Type = EventPhaseStart
	} else if status == "completed" {
		event.Type = EventPhaseComplete
	}
	pt.broadcaster.BroadcastEvent(pt.GetRoomID(), event)
}

func (pt *ProgressTracker) UpdateAgentStatus(agentName, status string, progress float64, message string) {
	pt.mu.Lock()
	pt.progress.UpdateAgentStatus(agentName, status, progress, message)
	pt.mu.Unlock()

	pt.broadcaster.BroadcastEvent(pt.GetRoomID(), ProgressEvent{
		Type: EventAgentStatus, Phase: pt.progress.Phase, Status: "active",
		Progress: pt.progress.Progress, Message: message,
		AgentStatus: map[string]AgentStatus{
			agentName: {Name: agentName, Status: status, Progress: progress, Message: message},
		},
	})
}

func (pt *ProgressTracker) SetError(err string) {
	pt.mu.Lock()
	pt.progress.SetError(err)
	pt.mu.Unlock()

	pt.broadcaster.BroadcastEvent(pt.GetRoomID(), ProgressEvent{
		Type: EventError, Phase: pt.progress.Phase, Status: "failed",
		Progress: pt.progress.Progress, Error: err, Message: "Review failed: " + err,
	})
}

func (pt *ProgressTracker) Complete() {
	pt.mu.Lock()
	pt.progress.Complete()
	pt.mu.Unlock()

	pt.broadcaster.BroadcastEvent(pt.GetRoomID(), ProgressEvent{
		Type: EventComplete, Phase: PhaseComplete, Status: "completed",
		Progress: 100, Message: "Review completed successfully",
	})
}

func (pt *ProgressTracker) ReportProgress(message string, progress float64) {
	pt.mu.Lock()
	pt.progress.UpdatePhase(PhaseAnalysis, "active", progress, message)
	pt.mu.Unlock()

	pt.broadcaster.BroadcastEvent(pt.GetRoomID(), ProgressEvent{
		Type: EventPhaseProgress, Phase: PhaseAnalysis, Status: "active",
		Progress: progress, Message: message,
	})
}

func (pt *ProgressTracker) ReportFileProgress(filename string, current, total int) {
	progress := float64(current) / float64(total) * 100
	pt.ReportProgress(fmt.Sprintf("Processing file %d/%d: %s", current, total, filename), progress)
}

func (pt *ProgressTracker) ReportAgentStatus(agentName, status, message string) {
	pt.UpdateAgentStatus(agentName, status, 0, message)
}
