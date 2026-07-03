package repository

import (
	"context"
	"time"

	"github.com/yi-nology/code-review-service/internal/domain/writeback"
	"github.com/yi-nology/code-review-service/internal/service/review/domain"
	ekitGorm "github.com/oekit/gormx" //lint:ignore SA1019 PageResponse 未废弃，上游包级别注释误标
)

// ReviewRepository 已迁移为 domain 类型（#6 keystone）：不再依赖 postgres/models。
// Request/Response 为强类型（domain.Request/Response），消除上层 JSONB 往返。
type ReviewRepository interface {
	Create(ctx context.Context, review *domain.Record) error
	Save(ctx context.Context, review *domain.Record) error
	GetByReviewID(ctx context.Context, reviewID string) (*domain.Record, error)
	GetByIdempotencyKey(ctx context.Context, key string) (*domain.Record, error)
	GetActiveByProviderRepo(ctx context.Context, provider, repo, pullRequestID, mergeRequestIID string) (*domain.Record, error)
	List(ctx context.Context, opts domain.ListOptions) (*ekitGorm.PageResponse[domain.Record], error)
	AggregateStats(ctx context.Context, opts domain.ListOptions) (*domain.AggregateStats, error)
	ProjectOCRMetrics(ctx context.Context, opts domain.ProjectStatsOptions) (*domain.ProjectOCRMetrics, error)
	ProjectOCRTrend(ctx context.Context, opts domain.ProjectStatsOptions) ([]domain.ProjectOCRTrendPoint, error)
	ClaimQueued(ctx context.Context, limit int) ([]domain.Record, error)
	RequeueStale(ctx context.Context, before time.Time) (int64, error)
	UpdateInTransaction(ctx context.Context, reviewID string, fn func(review *domain.Record) error) error
}

// WritebackItemRepository 已迁移为 domain 类型（#6）：不再依赖 postgres/models。
type WritebackItemRepository interface {
	Create(ctx context.Context, item *writeback.Item) error
	UpdateStatus(ctx context.Context, reviewID, findingKey, kind, status, externalID, providerURL, lastErr string) error
	IncrementAttempts(ctx context.Context, reviewID, findingKey, kind string) error
	ListByReview(ctx context.Context, reviewID string) ([]writeback.Item, error)
	ListRetryable(ctx context.Context, limit int) ([]writeback.Item, error)
	ListRecent(ctx context.Context, reviewID string, status string, limit int) ([]writeback.Item, error)
	CountByStatus(ctx context.Context, since time.Time) (map[string]int64, error)
}
