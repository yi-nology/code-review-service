package session

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/yi-nology/code-review-service/internal/repository/postgres"
	"github.com/yi-nology/code-review-service/internal/repository/postgres/models"
	"github.com/oekit/pkg/slicex"
	"github.com/cloudwego/hertz/pkg/common/hlog"
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

// Service defines the interface for session service operations
type Service interface {
	// CreateSession creates a new session for a review
	CreateSession(ctx context.Context, reviewID string, round int) (*models.ReviewSession, error)

	// GetSession returns a session by ID
	GetSession(ctx context.Context, sessionID string) (*models.ReviewSession, error)

	// GetActiveSession returns the active session for a review
	GetActiveSession(ctx context.Context, reviewID string) (*models.ReviewSession, error)

	// GetSessionsByReview returns all sessions for a review
	GetSessionsByReview(ctx context.Context, reviewID string) ([]*models.ReviewSession, error)

	// UpdatePhase updates the phase of a session
	UpdatePhase(ctx context.Context, sessionID string, phase models.SessionPhase, progress float64) error

	// UpdateStatus updates the status of a session
	UpdateStatus(ctx context.Context, sessionID string, status models.SessionStatus) error

	// UpdateAgentResults updates the agent results of a session
	UpdateAgentResults(ctx context.Context, sessionID string, agentResults []models.AgentResultData) error

	// UpdateDiscussion updates the discussion of a session
	UpdateDiscussion(ctx context.Context, sessionID string, discussion models.DiscussionData) error

	// UpdateFinalReport updates the final report of a session
	UpdateFinalReport(ctx context.Context, sessionID string, finalReport models.FinalReportData) error

	// CompleteSession marks a session as completed
	CompleteSession(ctx context.Context, sessionID string, finalReport models.FinalReportData) error

	// FailSession marks a session as failed
	FailSession(ctx context.Context, sessionID string, reason string) error

	// CancelSession cancels a session
	CancelSession(ctx context.Context, sessionID string) error

	// GetNextRound returns the next round number for a review
	GetNextRound(ctx context.Context, reviewID string) (int, error)

	// ListSessions returns sessions with pagination
	ListSessions(ctx context.Context, page, pageSize int) ([]*models.ReviewSession, int64, error)
}

// SessionService implements Service
type SessionService struct {
	repo postgres.SessionRepository
}

// NewSessionService creates a new session service
func NewSessionService(repo postgres.SessionRepository) *SessionService {
	return &SessionService{repo: repo}
}

// CreateSession creates a new session for a review
func (s *SessionService) CreateSession(ctx context.Context, reviewID string, round int) (*models.ReviewSession, error) {
	// Check if there's an active session
	activeSession, err := s.repo.GetActiveByReviewID(ctx, reviewID)
	if err != nil {
		return nil, fmt.Errorf("check active session: %w", err)
	}
	if activeSession != nil {
		return nil, fmt.Errorf("review %s already has an active session: %s", reviewID, activeSession.SessionID)
	}

	// Create new session
	session := &models.ReviewSession{
		SessionID: uuid.New().String(),
		ReviewID:  reviewID,
		Round:     round,
		Phase:     string(models.SessionPhaseContext),
		Status:    string(models.SessionStatusActive),
		Progress:  0,
		Metadata:  datatypes.JSON([]byte("{}")),
	}

	if err := s.repo.Create(ctx, session); err != nil {
		return nil, fmt.Errorf("create session: %w", err)
	}

	hlog.Infof("created session %s for review %s (round %d)", session.SessionID, reviewID, round)
	return session, nil
}

// GetSession returns a session by ID
func (s *SessionService) GetSession(ctx context.Context, sessionID string) (*models.ReviewSession, error) {
	return s.repo.GetByID(ctx, sessionID)
}

// GetActiveSession returns the active session for a review
func (s *SessionService) GetActiveSession(ctx context.Context, reviewID string) (*models.ReviewSession, error) {
	return s.repo.GetActiveByReviewID(ctx, reviewID)
}

// GetSessionsByReview returns all sessions for a review
func (s *SessionService) GetSessionsByReview(ctx context.Context, reviewID string) ([]*models.ReviewSession, error) {
	return s.repo.GetByReviewID(ctx, reviewID)
}

// UpdatePhase updates the phase of a session
func (s *SessionService) UpdatePhase(ctx context.Context, sessionID string, phase models.SessionPhase, progress float64) error {
	// Validate phase transition
	session, err := s.repo.GetByID(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("get session: %w", err)
	}

	if !isValidPhaseTransition(models.SessionPhase(session.Phase), phase) {
		return fmt.Errorf("invalid phase transition from %s to %s", session.Phase, phase)
	}

	if err := s.repo.UpdatePhase(ctx, sessionID, string(phase), progress); err != nil {
		return fmt.Errorf("update phase: %w", err)
	}

	hlog.Infof("session %s phase updated to %s (progress: %.1f%%)", sessionID, phase, progress)
	return nil
}

// UpdateStatus updates the status of a session
func (s *SessionService) UpdateStatus(ctx context.Context, sessionID string, status models.SessionStatus) error {
	if err := s.repo.UpdateStatus(ctx, sessionID, string(status)); err != nil {
		return fmt.Errorf("update status: %w", err)
	}

	hlog.Infof("session %s status updated to %s", sessionID, status)
	return nil
}

// UpdateAgentResults updates the agent results of a session
func (s *SessionService) UpdateAgentResults(ctx context.Context, sessionID string, agentResults []models.AgentResultData) error {
	if err := s.repo.UpdateAgentResults(ctx, sessionID, agentResults); err != nil {
		return fmt.Errorf("update agent results: %w", err)
	}

	hlog.Infof("session %s agent results updated (%d agents)", sessionID, len(agentResults))
	return nil
}

// UpdateDiscussion updates the discussion of a session
func (s *SessionService) UpdateDiscussion(ctx context.Context, sessionID string, discussion models.DiscussionData) error {
	if err := s.repo.UpdateDiscussion(ctx, sessionID, discussion); err != nil {
		return fmt.Errorf("update discussion: %w", err)
	}

	hlog.Infof("session %s discussion updated (%d entries)", sessionID, len(discussion.Entries))
	return nil
}

// UpdateFinalReport updates the final report of a session
func (s *SessionService) UpdateFinalReport(ctx context.Context, sessionID string, finalReport models.FinalReportData) error {
	if err := s.repo.UpdateFinalReport(ctx, sessionID, finalReport); err != nil {
		return fmt.Errorf("update final report: %w", err)
	}

	hlog.Infof("session %s final report updated", sessionID)
	return nil
}

// CompleteSession marks a session as completed
func (s *SessionService) CompleteSession(ctx context.Context, sessionID string, finalReport models.FinalReportData) error {
	if err := s.repo.Complete(ctx, sessionID, finalReport); err != nil {
		return fmt.Errorf("complete session: %w", err)
	}

	hlog.Infof("session %s completed", sessionID)
	return nil
}

// FailSession marks a session as failed
func (s *SessionService) FailSession(ctx context.Context, sessionID string, reason string) error {
	// Update metadata with error reason
	session, err := s.repo.GetByID(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("get session: %w", err)
	}

	metadata := make(map[string]interface{})
	if session.Metadata != nil {
		if err := json.Unmarshal(session.Metadata, &metadata); err != nil {
			return fmt.Errorf("unmarshal metadata: %w", err)
		}
	}
	metadata["error"] = reason
	metadata["failed_at"] = time.Now()

	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("marshal metadata: %w", err)
	}

	// Update session
	if err := s.repo.Update(ctx, &models.ReviewSession{
		SessionID: sessionID,
		Status:    string(models.SessionStatusFailed),
		Metadata:  metadataJSON,
	}); err != nil {
		return fmt.Errorf("fail session: %w", err)
	}

	hlog.Infof("session %s failed: %s", sessionID, reason)
	return nil
}

// CancelSession cancels a session
func (s *SessionService) CancelSession(ctx context.Context, sessionID string) error {
	if err := s.repo.UpdateStatus(ctx, sessionID, string(models.SessionStatusCancelled)); err != nil {
		return fmt.Errorf("cancel session: %w", err)
	}

	hlog.Infof("session %s cancelled", sessionID)
	return nil
}

// GetNextRound returns the next round number for a review
func (s *SessionService) GetNextRound(ctx context.Context, reviewID string) (int, error) {
	sessions, err := s.repo.GetByReviewID(ctx, reviewID)
	if err != nil {
		return 0, fmt.Errorf("get sessions: %w", err)
	}

	if len(sessions) == 0 {
		return 1, nil
	}

	// Find the highest round
	maxRound := 0
	for _, session := range sessions {
		if session.Round > maxRound {
			maxRound = session.Round
		}
	}

	return maxRound + 1, nil
}

// ListSessions returns sessions with pagination
func (s *SessionService) ListSessions(ctx context.Context, page, pageSize int) ([]*models.ReviewSession, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}

	offset := (page - 1) * pageSize
	return s.repo.List(ctx, offset, pageSize)
}

// isValidPhaseTransition checks if a phase transition is valid
func isValidPhaseTransition(from, to models.SessionPhase) bool {
	// Define valid transitions
	validTransitions := map[models.SessionPhase][]models.SessionPhase{
		models.SessionPhaseContext: {
			models.SessionPhaseChangeContext,
		},
		models.SessionPhaseChangeContext: {
			models.SessionPhaseAnalysis,
		},
		models.SessionPhaseAnalysis: {
			models.SessionPhaseReviews,
		},
		models.SessionPhaseReviews: {
			models.SessionPhaseAggregation,
		},
		models.SessionPhaseAggregation: {
			models.SessionPhaseDiscussion,
			models.SessionPhaseSynthesis, // Can skip discussion
		},
		models.SessionPhaseDiscussion: {
			models.SessionPhaseSynthesis,
		},
		models.SessionPhaseSynthesis: {
			models.SessionPhaseComplete,
		},
		models.SessionPhaseComplete: {},
	}

	allowed, ok := validTransitions[from]
	if !ok {
		return false
	}

	return slicex.Contains(allowed, to)
}
