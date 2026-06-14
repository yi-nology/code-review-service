package review_service

import (
	"context"
	"net/http"

	"github.com/yi-nology/code-review-service/internal/service/progress"
	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/hlog"
	"github.com/hertz-contrib/websocket"
)

// ProgressWSHandler handles WebSocket connections for progress updates
type ProgressWSHandler struct {
	progressManager *progress.ProgressManager
	upgrader        websocket.HertzUpgrader
}

// NewProgressWSHandler creates a new progress WebSocket handler
func NewProgressWSHandler(progressManager *progress.ProgressManager) *ProgressWSHandler {
	return &ProgressWSHandler{
		progressManager: progressManager,
		upgrader: websocket.HertzUpgrader{
			CheckOrigin: func(r *app.RequestContext) bool {
				// Allow all origins for development
				// In production, implement proper origin checking
				return true
			},
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
		},
	}
}

// HandleProgressWS handles WebSocket connections for progress updates
func (h *ProgressWSHandler) HandleProgressWS(ctx context.Context, c *app.RequestContext) {
	reviewID := c.Param("id")
	if reviewID == "" {
		c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error": "review ID is required",
		})
		return
	}

	// Upgrade HTTP connection to WebSocket
	var conn *websocket.Conn
	err := h.upgrader.Upgrade(c, func(ws *websocket.Conn) {
		conn = ws
	})
	if err != nil {
		hlog.Errorf("failed to upgrade websocket connection: %v", err)
		return
	}
	defer conn.Close()

	// Get or create progress tracker
	tracker := h.progressManager.GetTracker(reviewID)
	if tracker == nil {
		// Send error if no active review
		conn.WriteJSON(map[string]interface{}{
			"type":    "error",
			"error":   "no active review found",
			"message": "No active review found for this ID",
		})
		return
	}

	// Register connection to room
	roomID := tracker.GetRoomID()
	broadcaster := h.progressManager.GetBroadcaster()
	broadcaster.Register(roomID, conn)
	defer broadcaster.Unregister(roomID, conn)

	// Send current progress
	progress := tracker.GetProgress()
	if err := conn.WriteJSON(progress); err != nil {
		hlog.Errorf("failed to send initial progress: %v", err)
		return
	}

	hlog.Infof("websocket client connected for review: %s", reviewID)

	// Keep connection alive and handle disconnection
	for {
		// Read messages (just to detect disconnection)
		_, _, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				hlog.Errorf("websocket error: %v", err)
			}
			break
		}
	}

	hlog.Infof("websocket client disconnected for review: %s", reviewID)
}

// GetProgress returns the current progress for a review
func (h *ProgressWSHandler) GetProgress(ctx context.Context, c *app.RequestContext) {
	reviewID := c.Param("id")
	if reviewID == "" {
		c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error": "review ID is required",
		})
		return
	}

	tracker := h.progressManager.GetTracker(reviewID)
	if tracker == nil {
		c.JSON(http.StatusNotFound, map[string]interface{}{
			"error": "no active review found",
		})
		return
	}

	progress := tracker.GetProgress()
	c.JSON(http.StatusOK, progress)
}

// GetActiveReviews returns all active reviews with progress
func (h *ProgressWSHandler) GetActiveReviews(ctx context.Context, c *app.RequestContext) {
	trackers := h.progressManager.GetActiveTrackers()

	reviews := make([]map[string]interface{}, 0, len(trackers))
	for _, tracker := range trackers {
		progress := tracker.GetProgress()
		reviews = append(reviews, map[string]interface{}{
			"review_id":  progress.ReviewID,
			"session_id": progress.SessionID,
			"round":      progress.Round,
			"phase":      progress.Phase,
			"status":     progress.Status,
			"progress":   progress.Progress,
			"start_time": progress.StartTime,
			"updated_at": progress.UpdatedAt,
		})
	}

	c.JSON(http.StatusOK, map[string]interface{}{
		"reviews": reviews,
		"total":   len(reviews),
	})
}
