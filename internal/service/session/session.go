package session

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/yi-nology/code-review-service/internal/repository"
	"github.com/yi-nology/code-review-service/internal/repository/postgres/models"
	"github.com/yi-nology/code-review-service/internal/repository/repoerr"
	"github.com/yi-nology/code-review-service/internal/service/types"
	"github.com/oekit/gormx" //lint:ignore SA1019 PageResponse 未废弃，上游包级别注释误标
	"github.com/oekit/logx"
	"gorm.io/datatypes"
)

type SessionService struct {
	repo repository.SessionRepository
}

func NewSessionService(repo repository.SessionRepository) *SessionService {
	return &SessionService{repo: repo}
}

func (s *SessionService) CreateSession(ctx context.Context, reviewID string, round int) (*models.ReviewSession, error) {
	activeSession, err := s.repo.GetActiveByReviewID(ctx, reviewID)
	if err != nil && !errors.Is(err, repoerr.ErrNotFound) {
		return nil, fmt.Errorf("check active session: %w", err)
	}
	if activeSession != nil {
		return nil, fmt.Errorf("review %s already has an active session: %s", reviewID, activeSession.SessionID)
	}

	session := &models.ReviewSession{
		SessionID: types.GenerateID(),
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
	logx.Infof("created session %s for review %s (round %d)", session.SessionID, reviewID, round)
	return session, nil
}

func (s *SessionService) GetSession(ctx context.Context, sessionID string) (*models.ReviewSession, error) {
	session, err := s.repo.GetByID(ctx, sessionID)
	if err != nil {
		if errors.Is(err, repoerr.ErrNotFound) {
			return nil, fmt.Errorf("session not found: %s", sessionID)
		}
		return nil, err
	}
	return session, nil
}

func (s *SessionService) GetActiveSession(ctx context.Context, reviewID string) (*models.ReviewSession, error) {
	session, err := s.repo.GetActiveByReviewID(ctx, reviewID)
	if err != nil {
		if errors.Is(err, repoerr.ErrNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return session, nil
}

func (s *SessionService) GetSessionsByReview(ctx context.Context, reviewID string) ([]*models.ReviewSession, error) {
	return s.repo.GetByReviewID(ctx, reviewID)
}

func (s *SessionService) UpdatePhase(ctx context.Context, sessionID string, phase models.SessionPhase, progress float64) error {
	session, err := s.repo.GetByID(ctx, sessionID)
	if err != nil {
		if errors.Is(err, repoerr.ErrNotFound) {
			return fmt.Errorf("session not found: %s", sessionID)
		}
		return fmt.Errorf("get session: %w", err)
	}
	if !isValidPhaseTransition(models.SessionPhase(session.Phase), phase) {
		return fmt.Errorf("invalid phase transition from %s to %s", session.Phase, phase)
	}
	if err := s.repo.UpdatePhase(ctx, sessionID, string(phase), progress); err != nil {
		return fmt.Errorf("update phase: %w", err)
	}
	logx.Infof("session %s phase updated to %s (progress: %.1f%%)", sessionID, phase, progress)
	return nil
}

func (s *SessionService) UpdateStatus(ctx context.Context, sessionID string, status models.SessionStatus) error {
	if err := s.repo.UpdateStatus(ctx, sessionID, string(status)); err != nil {
		return fmt.Errorf("update status: %w", err)
	}
	logx.Infof("session %s status updated to %s", sessionID, status)
	return nil
}

func (s *SessionService) UpdateAgentResults(ctx context.Context, sessionID string, agentResults []models.AgentResultData) error {
	if err := s.repo.UpdateAgentResults(ctx, sessionID, agentResults); err != nil {
		return fmt.Errorf("update agent results: %w", err)
	}
	logx.Infof("session %s agent results updated (%d agents)", sessionID, len(agentResults))
	return nil
}

func (s *SessionService) UpdateDiscussion(ctx context.Context, sessionID string, discussion models.DiscussionData) error {
	if err := s.repo.UpdateDiscussion(ctx, sessionID, discussion); err != nil {
		return fmt.Errorf("update discussion: %w", err)
	}
	logx.Infof("session %s discussion updated (%d entries)", sessionID, len(discussion.Entries))
	return nil
}

func (s *SessionService) UpdateFinalReport(ctx context.Context, sessionID string, finalReport models.FinalReportData) error {
	if err := s.repo.UpdateFinalReport(ctx, sessionID, finalReport); err != nil {
		return fmt.Errorf("update final report: %w", err)
	}
	logx.Infof("session %s final report updated", sessionID)
	return nil
}

func (s *SessionService) CompleteSession(ctx context.Context, sessionID string, finalReport models.FinalReportData) error {
	if err := s.repo.Complete(ctx, sessionID, finalReport); err != nil {
		return fmt.Errorf("complete session: %w", err)
	}
	logx.Infof("session %s completed", sessionID)
	return nil
}

func (s *SessionService) FailSession(ctx context.Context, sessionID string, reason string) error {
	session, err := s.repo.GetByID(ctx, sessionID)
	if err != nil {
		if errors.Is(err, repoerr.ErrNotFound) {
			return fmt.Errorf("session not found: %s", sessionID)
		}
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
	if err := s.repo.Update(ctx, &models.ReviewSession{
		SessionID: sessionID, Status: string(models.SessionStatusFailed), Metadata: metadataJSON,
	}); err != nil {
		return fmt.Errorf("fail session: %w", err)
	}
	logx.Infof("session %s failed: %s", sessionID, reason)
	return nil
}

func (s *SessionService) CancelSession(ctx context.Context, sessionID string) error {
	if err := s.repo.UpdateStatus(ctx, sessionID, string(models.SessionStatusCancelled)); err != nil {
		return fmt.Errorf("cancel session: %w", err)
	}
	logx.Infof("session %s cancelled", sessionID)
	return nil
}

func (s *SessionService) GetNextRound(ctx context.Context, reviewID string) (int, error) {
	sessions, err := s.repo.GetByReviewID(ctx, reviewID)
	if err != nil {
		return 0, fmt.Errorf("get sessions: %w", err)
	}
	if len(sessions) == 0 {
		return 1, nil
	}
	maxRound := 0
	for _, s := range sessions {
		if s.Round > maxRound {
			maxRound = s.Round
		}
	}
	return maxRound + 1, nil
}

func (s *SessionService) ListSessions(ctx context.Context, page, pageSize int) (*gormx.PageResponse[models.ReviewSession], error) {
	return s.repo.List(ctx, page, pageSize)
}
