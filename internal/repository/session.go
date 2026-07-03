package repository

import (
	"context"

	"github.com/yi-nology/code-review-service/internal/domain/session"
	ekitGorm "github.com/oekit/gormx" //lint:ignore SA1019 PageResponse 未废弃，上游包级别注释误标
)

// SessionRepository 定义了 session 仓储的所有操作（#6：已迁移为 domain 类型）。
type SessionRepository interface {
	SessionReader
	SessionWriter
	SessionQuery
}

// SessionReader 定义了读取操作
type SessionReader interface {
	GetByID(ctx context.Context, sessionID string) (*session.Session, error)
	GetByReviewID(ctx context.Context, reviewID string) ([]*session.Session, error)
	GetActiveByReviewID(ctx context.Context, reviewID string) (*session.Session, error)
}

// SessionWriter 定义了写入操作。
// #6：原 4 个 interface{} 方法改为强类型 payload，消除反射/隐式类型泄漏。
type SessionWriter interface {
	Create(ctx context.Context, session *session.Session) error
	Update(ctx context.Context, session *session.Session) error
	UpdatePhase(ctx context.Context, sessionID string, phase string, progress float64) error
	UpdateStatus(ctx context.Context, sessionID string, status string) error
	UpdateAgentResults(ctx context.Context, sessionID string, agentResults []session.AgentResultData) error
	UpdateDiscussion(ctx context.Context, sessionID string, discussion session.DiscussionData) error
	UpdateFinalReport(ctx context.Context, sessionID string, finalReport session.FinalReportData) error
	Complete(ctx context.Context, sessionID string, finalReport session.FinalReportData) error
	Delete(ctx context.Context, sessionID string) error
	CancelActiveByReviewID(ctx context.Context, reviewID string) error
}

// SessionQuery 定义了查询操作
type SessionQuery interface {
	List(ctx context.Context, page, pageSize int) (*ekitGorm.PageResponse[session.Session], error)
}

// SessionCanceller 定义了取消操作（最小接口）
type SessionCanceller interface {
	CancelActiveByReviewID(ctx context.Context, reviewID string) error
}
