package repository

import (
	"context"

	"github.com/yi-nology/code-review-service/internal/repository/postgres/models"
	ekitGorm "github.com/oekit/gormx"
)

// SessionRepository 定义了 session 仓储的所有操作
type SessionRepository interface {
	SessionReader
	SessionWriter
	SessionQuery
}

// SessionReader 定义了读取操作
type SessionReader interface {
	GetByID(ctx context.Context, sessionID string) (*models.ReviewSession, error)
	GetByReviewID(ctx context.Context, reviewID string) ([]*models.ReviewSession, error)
	GetActiveByReviewID(ctx context.Context, reviewID string) (*models.ReviewSession, error)
}

// SessionWriter 定义了写入操作
type SessionWriter interface {
	Create(ctx context.Context, session *models.ReviewSession) error
	Update(ctx context.Context, session *models.ReviewSession) error
	UpdatePhase(ctx context.Context, sessionID string, phase string, progress float64) error
	UpdateStatus(ctx context.Context, sessionID string, status string) error
	UpdateAgentResults(ctx context.Context, sessionID string, agentResults interface{}) error
	UpdateDiscussion(ctx context.Context, sessionID string, discussion interface{}) error
	UpdateFinalReport(ctx context.Context, sessionID string, finalReport interface{}) error
	Complete(ctx context.Context, sessionID string, finalReport interface{}) error
	Delete(ctx context.Context, sessionID string) error
	CancelActiveByReviewID(ctx context.Context, reviewID string) error
}

// SessionQuery 定义了查询操作
type SessionQuery interface {
	List(ctx context.Context, page, pageSize int) (*ekitGorm.PageResponse[models.ReviewSession], error)
}

// SessionCanceller 定义了取消操作（最小接口）
type SessionCanceller interface {
	CancelActiveByReviewID(ctx context.Context, reviewID string) error
}
