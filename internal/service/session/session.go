package session

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	dsession "github.com/yi-nology/code-review-service/internal/domain/session"
	"github.com/yi-nology/code-review-service/internal/repository"
	"github.com/yi-nology/code-review-service/internal/repository/repoerr"
	"github.com/yi-nology/code-review-service/internal/service/types"
	"github.com/oekit/gormx" //lint:ignore SA1019 PageResponse 未废弃，上游包级别注释误标
	"github.com/oekit/logx"
)

type SessionService struct {
	repo repository.SessionRepository
}

func NewSessionService(repo repository.SessionRepository) *SessionService {
	return &SessionService{repo: repo}
}

func (s *SessionService) CreateSession(ctx context.Context, reviewID string, round int) (*dsession.Session, error) {
	activeSession, err := s.repo.GetActiveByReviewID(ctx, reviewID)
	if err != nil && !errors.Is(err, repoerr.ErrNotFound) {
		return nil, fmt.Errorf("check active session: %w", err)
	}
	if activeSession != nil {
		return nil, fmt.Errorf("review %s already has an active session: %s", reviewID, activeSession.SessionID)
	}

	sess := &dsession.Session{
		SessionID: types.GenerateID(),
		ReviewID:  reviewID,
		Round:     round,
		Phase:     dsession.PhaseContext,
		Status:    dsession.StatusActive,
		Progress:  0,
		Metadata:  json.RawMessage([]byte("{}")),
	}
	if err := s.repo.Create(ctx, sess); err != nil {
		return nil, fmt.Errorf("create session: %w", err)
	}
	logx.Infof("created session %s for review %s (round %d)", sess.SessionID, reviewID, round)
	return sess, nil
}

func (s *SessionService) GetSession(ctx context.Context, sessionID string) (*dsession.Session, error) {
	sess, err := s.repo.GetByID(ctx, sessionID)
	if err != nil {
		if errors.Is(err, repoerr.ErrNotFound) {
			return nil, fmt.Errorf("session not found: %s", sessionID)
		}
		return nil, err
	}
	return sess, nil
}

func (s *SessionService) GetActiveSession(ctx context.Context, reviewID string) (*dsession.Session, error) {
	sess, err := s.repo.GetActiveByReviewID(ctx, reviewID)
	if err != nil {
		if errors.Is(err, repoerr.ErrNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return sess, nil
}

func (s *SessionService) GetSessionsByReview(ctx context.Context, reviewID string) ([]*dsession.Session, error) {
	return s.repo.GetByReviewID(ctx, reviewID)
}

func (s *SessionService) UpdatePhase(ctx context.Context, sessionID string, phase string, progress float64) error {
	sess, err := s.repo.GetByID(ctx, sessionID)
	if err != nil {
		if errors.Is(err, repoerr.ErrNotFound) {
			return fmt.Errorf("session not found: %s", sessionID)
		}
		return fmt.Errorf("get session: %w", err)
	}
	if !isValidPhaseTransition(sess.Phase, phase) {
		return fmt.Errorf("invalid phase transition from %s to %s", sess.Phase, phase)
	}
	if err := s.repo.UpdatePhase(ctx, sessionID, phase, progress); err != nil {
		return fmt.Errorf("update phase: %w", err)
	}
	logx.Infof("session %s phase updated to %s (progress: %.1f%%)", sessionID, phase, progress)
	return nil
}

func (s *SessionService) UpdateStatus(ctx context.Context, sessionID string, status string) error {
	if err := s.repo.UpdateStatus(ctx, sessionID, status); err != nil {
		return fmt.Errorf("update status: %w", err)
	}
	logx.Infof("session %s status updated to %s", sessionID, status)
	return nil
}

func (s *SessionService) UpdateAgentResults(ctx context.Context, sessionID string, agentResults []dsession.AgentResultData) error {
	if err := s.repo.UpdateAgentResults(ctx, sessionID, agentResults); err != nil {
		return fmt.Errorf("update agent results: %w", err)
	}
	logx.Infof("session %s agent results updated (%d agents)", sessionID, len(agentResults))
	return nil
}

func (s *SessionService) UpdateDiscussion(ctx context.Context, sessionID string, discussion dsession.DiscussionData) error {
	if err := s.repo.UpdateDiscussion(ctx, sessionID, discussion); err != nil {
		return fmt.Errorf("update discussion: %w", err)
	}
	logx.Infof("session %s discussion updated (%d entries)", sessionID, len(discussion.Entries))
	return nil
}

func (s *SessionService) UpdateFinalReport(ctx context.Context, sessionID string, finalReport dsession.FinalReportData) error {
	if err := s.repo.UpdateFinalReport(ctx, sessionID, finalReport); err != nil {
		return fmt.Errorf("update final report: %w", err)
	}
	logx.Infof("session %s final report updated", sessionID)
	return nil
}

func (s *SessionService) CompleteSession(ctx context.Context, sessionID string, finalReport dsession.FinalReportData) error {
	if err := s.repo.Complete(ctx, sessionID, finalReport); err != nil {
		return fmt.Errorf("complete session: %w", err)
	}
	logx.Infof("session %s completed", sessionID)
	return nil
}

func (s *SessionService) FailSession(ctx context.Context, sessionID string, reason string) error {
	sess, err := s.repo.GetByID(ctx, sessionID)
	if err != nil {
		if errors.Is(err, repoerr.ErrNotFound) {
			return fmt.Errorf("session not found: %s", sessionID)
		}
		return fmt.Errorf("get session: %w", err)
	}
	metadata := make(map[string]interface{})
	if sess.Metadata != nil {
		if err := json.Unmarshal(sess.Metadata, &metadata); err != nil {
			return fmt.Errorf("unmarshal metadata: %w", err)
		}
	}
	metadata["error"] = reason
	metadata["failed_at"] = time.Now()
	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("marshal metadata: %w", err)
	}
	if err := s.repo.Update(ctx, &dsession.Session{
		SessionID: sessionID, Status: dsession.StatusFailed, Metadata: metadataJSON,
	}); err != nil {
		return fmt.Errorf("fail session: %w", err)
	}
	logx.Infof("session %s failed: %s", sessionID, reason)
	return nil
}

func (s *SessionService) CancelSession(ctx context.Context, sessionID string) error {
	if err := s.repo.UpdateStatus(ctx, sessionID, dsession.StatusCancelled); err != nil {
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
	for _, sess := range sessions {
		if sess.Round > maxRound {
			maxRound = sess.Round
		}
	}
	return maxRound + 1, nil
}

func (s *SessionService) ListSessions(ctx context.Context, page, pageSize int) (*gormx.PageResponse[dsession.Session], error) {
	return s.repo.List(ctx, page, pageSize)
}
