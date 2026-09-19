package session

import (
	"context"

	"github.com/yi-nology/code-review-service/internal/domain/page"

	dsession "github.com/yi-nology/code-review-service/internal/domain/session"
)

// Service 是 session 服务的公共契约（#6：已迁移为 domain 类型）。
type Service interface {
	CreateSession(ctx context.Context, reviewID string, round int) (*dsession.Session, error)
	GetSession(ctx context.Context, sessionID string) (*dsession.Session, error)
	GetActiveSession(ctx context.Context, reviewID string) (*dsession.Session, error)
	GetSessionsByReview(ctx context.Context, reviewID string) ([]*dsession.Session, error)
	UpdatePhase(ctx context.Context, sessionID string, phase string, progress float64) error
	UpdateStatus(ctx context.Context, sessionID string, status string) error
	UpdateAgentResults(ctx context.Context, sessionID string, agentResults []dsession.AgentResultData) error
	UpdateDiscussion(ctx context.Context, sessionID string, discussion dsession.DiscussionData) error
	UpdateFinalReport(ctx context.Context, sessionID string, finalReport dsession.FinalReportData) error
	CompleteSession(ctx context.Context, sessionID string, finalReport dsession.FinalReportData) error
	FailSession(ctx context.Context, sessionID string, reason string) error
	CancelSession(ctx context.Context, sessionID string) error
	GetNextRound(ctx context.Context, reviewID string) (int, error)
	ListSessions(ctx context.Context, pageNum, pageSize int) (*page.Page[dsession.Session], error)
}
