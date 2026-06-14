package review_service

import (
	"context"
	"strconv"

	"github.com/yi-nology/code-review-service/internal/repository/postgres/models"
	"github.com/yi-nology/code-review-service/internal/service/session"
	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/hlog"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

// SessionHandler handles session-related HTTP requests
type SessionHandler struct {
	sessionService session.Service
}

// NewSessionHandler creates a new session handler
func NewSessionHandler(sessionService session.Service) *SessionHandler {
	return &SessionHandler{
		sessionService: sessionService,
	}
}

// GetSessions returns sessions for a review
func (h *SessionHandler) GetSessions(ctx context.Context, c *app.RequestContext) {
	reviewID := c.Param("id")
	if reviewID == "" {
		c.JSON(consts.StatusBadRequest, map[string]interface{}{
			"error": "review ID is required",
		})
		return
	}

	sessions, err := h.sessionService.GetSessionsByReview(ctx, reviewID)
	if err != nil {
		hlog.Errorf("get sessions failed: %v", err)
		c.JSON(consts.StatusInternalServerError, map[string]interface{}{
			"error": "failed to get sessions",
		})
		return
	}

	c.JSON(consts.StatusOK, map[string]interface{}{
		"sessions": sessions,
	})
}

// GetSession returns a session by ID
func (h *SessionHandler) GetSession(ctx context.Context, c *app.RequestContext) {
	sessionID := c.Param("sessionId")
	if sessionID == "" {
		c.JSON(consts.StatusBadRequest, map[string]interface{}{
			"error": "session ID is required",
		})
		return
	}

	session, err := h.sessionService.GetSession(ctx, sessionID)
	if err != nil {
		hlog.Errorf("get session failed: %v", err)
		c.JSON(consts.StatusNotFound, map[string]interface{}{
			"error": "session not found",
		})
		return
	}

	c.JSON(consts.StatusOK, session)
}

// GetActiveSession returns the active session for a review
func (h *SessionHandler) GetActiveSession(ctx context.Context, c *app.RequestContext) {
	reviewID := c.Param("id")
	if reviewID == "" {
		c.JSON(consts.StatusBadRequest, map[string]interface{}{
			"error": "review ID is required",
		})
		return
	}

	session, err := h.sessionService.GetActiveSession(ctx, reviewID)
	if err != nil {
		hlog.Errorf("get active session failed: %v", err)
		c.JSON(consts.StatusInternalServerError, map[string]interface{}{
			"error": "failed to get active session",
		})
		return
	}

	if session == nil {
		c.JSON(consts.StatusNotFound, map[string]interface{}{
			"error": "no active session found",
		})
		return
	}

	c.JSON(consts.StatusOK, session)
}

// CreateSession creates a new session for a review
func (h *SessionHandler) CreateSession(ctx context.Context, c *app.RequestContext) {
	reviewID := c.Param("id")
	if reviewID == "" {
		c.JSON(consts.StatusBadRequest, map[string]interface{}{
			"error": "review ID is required",
		})
		return
	}

	// Get next round number
	round, err := h.sessionService.GetNextRound(ctx, reviewID)
	if err != nil {
		hlog.Errorf("get next round failed: %v", err)
		c.JSON(consts.StatusInternalServerError, map[string]interface{}{
			"error": "failed to get next round",
		})
		return
	}

	// Create session
	session, err := h.sessionService.CreateSession(ctx, reviewID, round)
	if err != nil {
		hlog.Errorf("create session failed: %v", err)
		c.JSON(consts.StatusInternalServerError, map[string]interface{}{
			"error": "failed to create session",
		})
		return
	}

	c.JSON(consts.StatusCreated, session)
}

// UpdateSessionPhase updates the phase of a session
func (h *SessionHandler) UpdateSessionPhase(ctx context.Context, c *app.RequestContext) {
	sessionID := c.Param("sessionId")
	if sessionID == "" {
		c.JSON(consts.StatusBadRequest, map[string]interface{}{
			"error": "session ID is required",
		})
		return
	}

	var request struct {
		Phase    string  `json:"phase" binding:"required"`
		Progress float64 `json:"progress"`
	}

	if err := c.BindJSON(&request); err != nil {
		c.JSON(consts.StatusBadRequest, map[string]interface{}{
			"error": "invalid request body",
		})
		return
	}

	if err := h.sessionService.UpdatePhase(ctx, sessionID, models.SessionPhase(request.Phase), request.Progress); err != nil {
		hlog.Errorf("update session phase failed: %v", err)
		c.JSON(consts.StatusInternalServerError, map[string]interface{}{
			"error": "failed to update session phase",
		})
		return
	}

	c.JSON(consts.StatusOK, map[string]interface{}{
		"message": "session phase updated",
	})
}

// UpdateSessionStatus updates the status of a session
func (h *SessionHandler) UpdateSessionStatus(ctx context.Context, c *app.RequestContext) {
	sessionID := c.Param("sessionId")
	if sessionID == "" {
		c.JSON(consts.StatusBadRequest, map[string]interface{}{
			"error": "session ID is required",
		})
		return
	}

	var request struct {
		Status string `json:"status" binding:"required"`
	}

	if err := c.BindJSON(&request); err != nil {
		c.JSON(consts.StatusBadRequest, map[string]interface{}{
			"error": "invalid request body",
		})
		return
	}

	if err := h.sessionService.UpdateStatus(ctx, sessionID, models.SessionStatus(request.Status)); err != nil {
		hlog.Errorf("update session status failed: %v", err)
		c.JSON(consts.StatusInternalServerError, map[string]interface{}{
			"error": "failed to update session status",
		})
		return
	}

	c.JSON(consts.StatusOK, map[string]interface{}{
		"message": "session status updated",
	})
}

// UpdateSessionAgentResults updates the agent results of a session
func (h *SessionHandler) UpdateSessionAgentResults(ctx context.Context, c *app.RequestContext) {
	sessionID := c.Param("sessionId")
	if sessionID == "" {
		c.JSON(consts.StatusBadRequest, map[string]interface{}{
			"error": "session ID is required",
		})
		return
	}

	var agentResults []models.AgentResultData
	if err := c.BindJSON(&agentResults); err != nil {
		c.JSON(consts.StatusBadRequest, map[string]interface{}{
			"error": "invalid request body",
		})
		return
	}

	if err := h.sessionService.UpdateAgentResults(ctx, sessionID, agentResults); err != nil {
		hlog.Errorf("update session agent results failed: %v", err)
		c.JSON(consts.StatusInternalServerError, map[string]interface{}{
			"error": "failed to update session agent results",
		})
		return
	}

	c.JSON(consts.StatusOK, map[string]interface{}{
		"message": "session agent results updated",
	})
}

// UpdateSessionDiscussion updates the discussion of a session
func (h *SessionHandler) UpdateSessionDiscussion(ctx context.Context, c *app.RequestContext) {
	sessionID := c.Param("sessionId")
	if sessionID == "" {
		c.JSON(consts.StatusBadRequest, map[string]interface{}{
			"error": "session ID is required",
		})
		return
	}

	var discussion models.DiscussionData
	if err := c.BindJSON(&discussion); err != nil {
		c.JSON(consts.StatusBadRequest, map[string]interface{}{
			"error": "invalid request body",
		})
		return
	}

	if err := h.sessionService.UpdateDiscussion(ctx, sessionID, discussion); err != nil {
		hlog.Errorf("update session discussion failed: %v", err)
		c.JSON(consts.StatusInternalServerError, map[string]interface{}{
			"error": "failed to update session discussion",
		})
		return
	}

	c.JSON(consts.StatusOK, map[string]interface{}{
		"message": "session discussion updated",
	})
}

// UpdateSessionFinalReport updates the final report of a session
func (h *SessionHandler) UpdateSessionFinalReport(ctx context.Context, c *app.RequestContext) {
	sessionID := c.Param("sessionId")
	if sessionID == "" {
		c.JSON(consts.StatusBadRequest, map[string]interface{}{
			"error": "session ID is required",
		})
		return
	}

	var finalReport models.FinalReportData
	if err := c.BindJSON(&finalReport); err != nil {
		c.JSON(consts.StatusBadRequest, map[string]interface{}{
			"error": "invalid request body",
		})
		return
	}

	if err := h.sessionService.UpdateFinalReport(ctx, sessionID, finalReport); err != nil {
		hlog.Errorf("update session final report failed: %v", err)
		c.JSON(consts.StatusInternalServerError, map[string]interface{}{
			"error": "failed to update session final report",
		})
		return
	}

	c.JSON(consts.StatusOK, map[string]interface{}{
		"message": "session final report updated",
	})
}

// CompleteSession marks a session as completed
func (h *SessionHandler) CompleteSession(ctx context.Context, c *app.RequestContext) {
	sessionID := c.Param("sessionId")
	if sessionID == "" {
		c.JSON(consts.StatusBadRequest, map[string]interface{}{
			"error": "session ID is required",
		})
		return
	}

	var finalReport models.FinalReportData
	if err := c.BindJSON(&finalReport); err != nil {
		c.JSON(consts.StatusBadRequest, map[string]interface{}{
			"error": "invalid request body",
		})
		return
	}

	if err := h.sessionService.CompleteSession(ctx, sessionID, finalReport); err != nil {
		hlog.Errorf("complete session failed: %v", err)
		c.JSON(consts.StatusInternalServerError, map[string]interface{}{
			"error": "failed to complete session",
		})
		return
	}

	c.JSON(consts.StatusOK, map[string]interface{}{
		"message": "session completed",
	})
}

// FailSession marks a session as failed
func (h *SessionHandler) FailSession(ctx context.Context, c *app.RequestContext) {
	sessionID := c.Param("sessionId")
	if sessionID == "" {
		c.JSON(consts.StatusBadRequest, map[string]interface{}{
			"error": "session ID is required",
		})
		return
	}

	var request struct {
		Reason string `json:"reason" binding:"required"`
	}

	if err := c.BindJSON(&request); err != nil {
		c.JSON(consts.StatusBadRequest, map[string]interface{}{
			"error": "invalid request body",
		})
		return
	}

	if err := h.sessionService.FailSession(ctx, sessionID, request.Reason); err != nil {
		hlog.Errorf("fail session failed: %v", err)
		c.JSON(consts.StatusInternalServerError, map[string]interface{}{
			"error": "failed to fail session",
		})
		return
	}

	c.JSON(consts.StatusOK, map[string]interface{}{
		"message": "session failed",
	})
}

// CancelSession cancels a session
func (h *SessionHandler) CancelSession(ctx context.Context, c *app.RequestContext) {
	sessionID := c.Param("sessionId")
	if sessionID == "" {
		c.JSON(consts.StatusBadRequest, map[string]interface{}{
			"error": "session ID is required",
		})
		return
	}

	if err := h.sessionService.CancelSession(ctx, sessionID); err != nil {
		hlog.Errorf("cancel session failed: %v", err)
		c.JSON(consts.StatusInternalServerError, map[string]interface{}{
			"error": "failed to cancel session",
		})
		return
	}

	c.JSON(consts.StatusOK, map[string]interface{}{
		"message": "session cancelled",
	})
}

// ListSessions returns sessions with pagination
func (h *SessionHandler) ListSessions(ctx context.Context, c *app.RequestContext) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	sessions, total, err := h.sessionService.ListSessions(ctx, page, pageSize)
	if err != nil {
		hlog.Errorf("list sessions failed: %v", err)
		c.JSON(consts.StatusInternalServerError, map[string]interface{}{
			"error": "failed to list sessions",
		})
		return
	}

	c.JSON(consts.StatusOK, map[string]interface{}{
		"sessions":   sessions,
		"total":      total,
		"page":       page,
		"page_size":  pageSize,
		"total_pages": (total + int64(pageSize) - 1) / int64(pageSize),
	})
}
