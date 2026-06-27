package session

import (
	"context"

	"github.com/yi-nology/code-review-service/internal/repository/postgres/models"
	"github.com/oekit/gormx" //lint:ignore SA1019 PageResponse 未废弃，上游包级别注释误标
)

type Service interface {
	CreateSession(ctx context.Context, reviewID string, round int) (*models.ReviewSession, error)
	GetSession(ctx context.Context, sessionID string) (*models.ReviewSession, error)
	GetActiveSession(ctx context.Context, reviewID string) (*models.ReviewSession, error)
	GetSessionsByReview(ctx context.Context, reviewID string) ([]*models.ReviewSession, error)
	UpdatePhase(ctx context.Context, sessionID string, phase models.SessionPhase, progress float64) error
	UpdateStatus(ctx context.Context, sessionID string, status models.SessionStatus) error
	UpdateAgentResults(ctx context.Context, sessionID string, agentResults []models.AgentResultData) error
	UpdateDiscussion(ctx context.Context, sessionID string, discussion models.DiscussionData) error
	UpdateFinalReport(ctx context.Context, sessionID string, finalReport models.FinalReportData) error
	CompleteSession(ctx context.Context, sessionID string, finalReport models.FinalReportData) error
	FailSession(ctx context.Context, sessionID string, reason string) error
	CancelSession(ctx context.Context, sessionID string) error
	GetNextRound(ctx context.Context, reviewID string) (int, error)
	ListSessions(ctx context.Context, page, pageSize int) (*gormx.PageResponse[models.ReviewSession], error)
}
