package progress

import (
	"context"
	"sync"
	"time"

	"github.com/oekit/logx"
	"github.com/yi-nology/agent-harness/eventbus"
)

type ProgressReporter interface {
	ReportProgress(message string, progress float64)
	ReportFileProgress(filename string, current, total int)
	ReportAgentStatus(agentName, status, message string)
	UpdatePhase(phase Phase, status string, progress float64, message string)
}

type progressReporterKey struct{}

func WithProgressReporter(ctx context.Context, reporter ProgressReporter) context.Context {
	return context.WithValue(ctx, progressReporterKey{}, reporter)
}

func ProgressReporterFromContext(ctx context.Context) ProgressReporter {
	if reporter, ok := ctx.Value(progressReporterKey{}).(ProgressReporter); ok {
		return reporter
	}
	return nil
}

type ProgressManager struct {
	broadcaster *broadcaster
	bus         *eventbus.Bus
	bridge      *eventBridge
	trackers    map[string]*ProgressTracker
	mu          sync.RWMutex
}

const completedTrackerRetention = 10 * time.Minute

func NewProgressManager() *ProgressManager {
	bc := newBroadcaster()
	bus := eventbus.New()
	pm := &ProgressManager{
		broadcaster: bc,
		bus:         bus,
		trackers:    make(map[string]*ProgressTracker),
	}
	pm.bridge = newEventBridge(context.Background(), bus, bc)
	return pm
}

// Bus 返回进程内的事件总线（底座 agent-harness/eventbus）。
//
// 编排层（workflow）用它发布阶段生命周期事件；前端 websocket 订阅的是
// 桥接出去的进度事件。两者共用一条总线，事件面因此只有一个发布点。
func (pm *ProgressManager) Bus() *eventbus.Bus {
	return pm.bus
}

// GetBroadcaster 返回 Broadcaster 接口（PR-12 修复：原返回 unexported *broadcaster，
// 现返回 Broadcaster 接口便于测试 mock）。
func (pm *ProgressManager) GetBroadcaster() Broadcaster {
	return pm.broadcaster
}

func (pm *ProgressManager) GetTracker(reviewID string) *ProgressTracker {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return pm.trackers[reviewID]
}

func (pm *ProgressManager) CreateTracker(reviewID, sessionID string, round int) *ProgressTracker {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if existing, ok := pm.trackers[reviewID]; ok {
		existing.Complete()
	}

	tracker := newProgressTracker(pm.broadcaster, pm.bus, reviewID, sessionID, round)
	pm.trackers[reviewID] = tracker
	logx.Infof("created progress tracker for review: %s", reviewID)
	return tracker
}

func (pm *ProgressManager) RemoveTracker(reviewID string) {
	pm.mu.Lock()
	tracker, ok := pm.trackers[reviewID]
	if ok {
		if tracker.GetProgress().IsActive() {
			tracker.Complete()
		}
		logx.Infof("retired progress tracker for review: %s", reviewID)
	}
	pm.mu.Unlock()

	if !ok {
		return
	}

	time.AfterFunc(completedTrackerRetention, func() {
		pm.mu.Lock()
		defer pm.mu.Unlock()
		if pm.trackers[reviewID] == tracker {
			delete(pm.trackers, reviewID)
			logx.Infof("removed retained progress tracker for review: %s", reviewID)
		}
	})
}

func (pm *ProgressManager) GetActiveTrackers() []*ProgressTracker {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	trackers := make([]*ProgressTracker, 0, len(pm.trackers))
	for _, tracker := range pm.trackers {
		if tracker.GetProgress().IsActive() {
			trackers = append(trackers, tracker)
		}
	}
	return trackers
}

func (pm *ProgressManager) Close() {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	for _, tracker := range pm.trackers {
		tracker.Complete()
	}
	pm.trackers = make(map[string]*ProgressTracker)
	if pm.bridge != nil {
		pm.bridge.Close()
	}
	pm.broadcaster.Close()
}
