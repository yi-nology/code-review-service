package progress

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/cloudwego/hertz/pkg/common/hlog"
	"github.com/hertz-contrib/websocket"
)

// Broadcaster manages WebSocket connections and broadcasts progress events
type Broadcaster struct {
	rooms      map[string]map[*websocket.Conn]bool
	mu         sync.RWMutex
	register   chan *registration
	unregister chan *registration
	broadcast  chan *broadcastMessage
	stop       chan struct{}
}

// registration represents a WebSocket registration/unregistration
type registration struct {
	room string
	conn *websocket.Conn
}

// broadcastMessage represents a message to broadcast
type broadcastMessage struct {
	room    string
	message []byte
}

// NewBroadcaster creates a new broadcaster
func NewBroadcaster() *Broadcaster {
	b := &Broadcaster{
		rooms:      make(map[string]map[*websocket.Conn]bool),
		register:   make(chan *registration),
		unregister: make(chan *registration),
		broadcast:  make(chan *broadcastMessage),
		stop:       make(chan struct{}),
	}

	go b.run()
	return b
}

// run handles broadcaster events
func (b *Broadcaster) run() {
	for {
		select {
		case reg := <-b.register:
			b.handleRegister(reg)
		case reg := <-b.unregister:
			b.handleUnregister(reg)
		case msg := <-b.broadcast:
			b.handleBroadcast(msg)
		case <-b.stop:
			return
		}
	}
}

// handleRegister handles a new WebSocket connection
func (b *Broadcaster) handleRegister(reg *registration) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if _, ok := b.rooms[reg.room]; !ok {
		b.rooms[reg.room] = make(map[*websocket.Conn]bool)
	}
	b.rooms[reg.room][reg.conn] = true

	hlog.Infof("websocket client connected to room: %s (total: %d)", reg.room, len(b.rooms[reg.room]))
}

// handleUnregister handles a WebSocket disconnection
func (b *Broadcaster) handleUnregister(reg *registration) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if conns, ok := b.rooms[reg.room]; ok {
		if _, ok := conns[reg.conn]; ok {
			delete(conns, reg.conn)
			reg.conn.Close()

			hlog.Infof("websocket client disconnected from room: %s (total: %d)", reg.room, len(conns))

			// Clean up empty rooms
			if len(conns) == 0 {
				delete(b.rooms, reg.room)
			}
		}
	}
}

// handleBroadcast handles broadcasting a message to a room
func (b *Broadcaster) handleBroadcast(msg *broadcastMessage) {
	b.mu.RLock()
	conns, ok := b.rooms[msg.room]
	if !ok {
		b.mu.RUnlock()
		return
	}

	// Copy connections to avoid holding lock during write
	connections := make([]*websocket.Conn, 0, len(conns))
	for conn := range conns {
		connections = append(connections, conn)
	}
	b.mu.RUnlock()

	// Send message to all connections
	for _, conn := range connections {
		if err := conn.WriteMessage(websocket.TextMessage, msg.message); err != nil {
			hlog.Errorf("failed to send websocket message: %v", err)
			b.Unregister(msg.room, conn)
		}
	}
}

// Register registers a WebSocket connection to a room
func (b *Broadcaster) Register(room string, conn *websocket.Conn) {
	b.register <- &registration{room: room, conn: conn}
}

// Unregister unregisters a WebSocket connection from a room
func (b *Broadcaster) Unregister(room string, conn *websocket.Conn) {
	b.unregister <- &registration{room: room, conn: conn}
}

// Broadcast broadcasts a message to a room
func (b *Broadcaster) Broadcast(room string, message []byte) {
	b.broadcast <- &broadcastMessage{room: room, message: message}
}

// BroadcastEvent broadcasts a progress event to a room
func (b *Broadcaster) BroadcastEvent(room string, event ProgressEvent) {
	event.Timestamp = time.Now()
	data, err := json.Marshal(event)
	if err != nil {
		hlog.Errorf("failed to marshal progress event: %v", err)
		return
	}
	b.Broadcast(room, data)
}

// GetRoomCount returns the number of connections in a room
func (b *Broadcaster) GetRoomCount(room string) int {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if conns, ok := b.rooms[room]; ok {
		return len(conns)
	}
	return 0
}

// GetTotalConnections returns the total number of connections
func (b *Broadcaster) GetTotalConnections() int {
	b.mu.RLock()
	defer b.mu.RUnlock()

	total := 0
	for _, conns := range b.rooms {
		total += len(conns)
	}
	return total
}

// Close closes the broadcaster
func (b *Broadcaster) Close() {
	close(b.stop)

	b.mu.Lock()
	defer b.mu.Unlock()

	// Close all connections
	for room, conns := range b.rooms {
		for conn := range conns {
			conn.Close()
		}
		delete(b.rooms, room)
	}
}

// ProgressTracker tracks progress for a review and broadcasts updates
type ProgressTracker struct {
	broadcaster *Broadcaster
	reviewID    string
	sessionID   string
	progress    *ReviewProgress
	mu          sync.RWMutex
}

// NewProgressTracker creates a new progress tracker
func NewProgressTracker(broadcaster *Broadcaster, reviewID, sessionID string, round int) *ProgressTracker {
	return &ProgressTracker{
		broadcaster: broadcaster,
		reviewID:    reviewID,
		sessionID:   sessionID,
		progress:    NewReviewProgress(reviewID, sessionID, round),
	}
}

// GetRoomID returns the room ID for the review
func (pt *ProgressTracker) GetRoomID() string {
	return "review:" + pt.reviewID
}

// GetProgress returns the current progress
func (pt *ProgressTracker) GetProgress() *ReviewProgress {
	pt.mu.RLock()
	defer pt.mu.RUnlock()
	return pt.progress
}

// UpdatePhase updates the phase and broadcasts the update
func (pt *ProgressTracker) UpdatePhase(phase Phase, status string, progress float64, message string) {
	pt.mu.Lock()
	pt.progress.UpdatePhase(phase, status, progress, message)
	pt.mu.Unlock()

	// Broadcast phase update
	event := ProgressEvent{
		Type:     EventPhaseProgress,
		Phase:    phase,
		Status:   status,
		Progress: progress,
		Message:  message,
	}

	if status == "active" {
		event.Type = EventPhaseStart
	} else if status == "completed" {
		event.Type = EventPhaseComplete
	}

	pt.broadcaster.BroadcastEvent(pt.GetRoomID(), event)
}

// UpdateAgentStatus updates an agent's status and broadcasts the update
func (pt *ProgressTracker) UpdateAgentStatus(agentName, status string, progress float64, message string) {
	pt.mu.Lock()
	pt.progress.UpdateAgentStatus(agentName, status, progress, message)
	pt.mu.Unlock()

	// Broadcast agent status update
	event := ProgressEvent{
		Type:     EventAgentStatus,
		Phase:    pt.progress.Phase,
		Status:   "active",
		Progress: pt.progress.Progress,
		Message:  message,
		AgentStatus: map[string]AgentStatus{
			agentName: {
				Name:     agentName,
				Status:   status,
				Progress: progress,
				Message:  message,
			},
		},
	}

	pt.broadcaster.BroadcastEvent(pt.GetRoomID(), event)
}

// SetError sets an error and broadcasts the update
func (pt *ProgressTracker) SetError(err string) {
	pt.mu.Lock()
	pt.progress.SetError(err)
	pt.mu.Unlock()

	// Broadcast error
	event := ProgressEvent{
		Type:     EventError,
		Phase:    pt.progress.Phase,
		Status:   "failed",
		Progress: pt.progress.Progress,
		Error:    err,
		Message:  "Review failed: " + err,
	}

	pt.broadcaster.BroadcastEvent(pt.GetRoomID(), event)
}

// Complete marks the progress as complete and broadcasts the update
func (pt *ProgressTracker) Complete() {
	pt.mu.Lock()
	pt.progress.Complete()
	pt.mu.Unlock()

	// Broadcast completion
	event := ProgressEvent{
		Type:     EventComplete,
		Phase:    PhaseComplete,
		Status:   "completed",
		Progress: 100,
		Message:  "Review completed successfully",
	}

	pt.broadcaster.BroadcastEvent(pt.GetRoomID(), event)
}

// ReportProgress implements ProgressReporter interface
func (pt *ProgressTracker) ReportProgress(message string, progress float64) {
	pt.mu.Lock()
	pt.progress.UpdatePhase(PhaseAnalysis, "active", progress, message)
	pt.mu.Unlock()

	event := ProgressEvent{
		Type:     EventPhaseProgress,
		Phase:    PhaseAnalysis,
		Status:   "active",
		Progress: progress,
		Message:  message,
	}
	pt.broadcaster.BroadcastEvent(pt.GetRoomID(), event)
}

// ReportFileProgress implements ProgressReporter interface
func (pt *ProgressTracker) ReportFileProgress(filename string, current, total int) {
	progress := float64(current) / float64(total) * 100
	message := fmt.Sprintf("Processing file %d/%d: %s", current, total, filename)
	pt.ReportProgress(message, progress)
}

// ReportAgentStatus implements ProgressReporter interface
func (pt *ProgressTracker) ReportAgentStatus(agentName, status, message string) {
	pt.UpdateAgentStatus(agentName, status, 0, message)
}

// ProgressReporter is an interface for reporting OCR progress
type ProgressReporter interface {
	// ReportProgress reports OCR progress with a message and percentage
	ReportProgress(message string, progress float64)
	// ReportFileProgress reports progress for a specific file
 ReportFileProgress(filename string, current, total int)
	// ReportAgentStatus reports agent execution status
	ReportAgentStatus(agentName, status, message string)
}

// context key for progress reporter
type progressReporterKey struct{}

// WithProgressReporter adds a ProgressReporter to the context
func WithProgressReporter(ctx context.Context, reporter ProgressReporter) context.Context {
	return context.WithValue(ctx, progressReporterKey{}, reporter)
}

// ProgressReporterFromContext extracts the ProgressReporter from context
func ProgressReporterFromContext(ctx context.Context) ProgressReporter {
	if reporter, ok := ctx.Value(progressReporterKey{}).(ProgressReporter); ok {
		return reporter
	}
	return nil
}

// ProgressManager manages progress trackers for multiple reviews
type ProgressManager struct {
	broadcaster *Broadcaster
	trackers    map[string]*ProgressTracker
	mu          sync.RWMutex
}

// NewProgressManager creates a new progress manager
func NewProgressManager() *ProgressManager {
	return &ProgressManager{
		broadcaster: NewBroadcaster(),
		trackers:    make(map[string]*ProgressTracker),
	}
}

// GetBroadcaster returns the broadcaster
func (pm *ProgressManager) GetBroadcaster() *Broadcaster {
	return pm.broadcaster
}

// GetTracker returns the progress tracker for a review
func (pm *ProgressManager) GetTracker(reviewID string) *ProgressTracker {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return pm.trackers[reviewID]
}

// CreateTracker creates a new progress tracker for a review
func (pm *ProgressManager) CreateTracker(reviewID, sessionID string, round int) *ProgressTracker {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	// Close existing tracker if any
	if existing, ok := pm.trackers[reviewID]; ok {
		existing.Complete()
	}

	tracker := NewProgressTracker(pm.broadcaster, reviewID, sessionID, round)
	pm.trackers[reviewID] = tracker

	hlog.Infof("created progress tracker for review: %s", reviewID)
	return tracker
}

// RemoveTracker removes a progress tracker
func (pm *ProgressManager) RemoveTracker(reviewID string) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if tracker, ok := pm.trackers[reviewID]; ok {
		tracker.Complete()
		delete(pm.trackers, reviewID)
		hlog.Infof("removed progress tracker for review: %s", reviewID)
	}
}

// GetActiveTrackers returns all active trackers
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

// Close closes the progress manager
func (pm *ProgressManager) Close() {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	for _, tracker := range pm.trackers {
		tracker.Complete()
	}
	pm.trackers = make(map[string]*ProgressTracker)

	pm.broadcaster.Close()
}
