package repository

import (
	"context"
	"time"

	"github.com/yi-nology/code-review-service/internal/repository/postgres/models"
	ekitGorm "github.com/oekit/gormx"
)

type ReviewRepository interface {
	Create(ctx context.Context, review *models.Review) error
	Save(ctx context.Context, review *models.Review) error
	GetByReviewID(ctx context.Context, reviewID string) (*models.Review, error)
	GetByidempotencyKey(ctx context.Context, key string) (*models.Review, error)
	GetActiveByProviderRepo(ctx context.Context, provider, repo string) (*models.Review, error)
	List(ctx context.Context, opts models.ListOptions) (*ekitGorm.PageResponse[models.Review], error)
	AggregateStats(ctx context.Context, opts models.ListOptions) (*models.AggregateStats, error)
	ClaimQueued(ctx context.Context, limit int) ([]models.Review, error)
	RequeueStale(ctx context.Context, before time.Time) (int64, error)
	UpdateInTransaction(ctx context.Context, reviewID string, fn func(review *models.Review) error) error
}

type WritebackItemRepository interface {
	Create(ctx context.Context, item *models.WritebackItem) error
	UpdateStatus(ctx context.Context, reviewID, findingKey, kind, status, externalID, providerURL, lastErr string) error
	IncrementAttempts(ctx context.Context, reviewID, findingKey, kind string) error
	ListByReview(ctx context.Context, reviewID string) ([]models.WritebackItem, error)
	ListRetryable(ctx context.Context, limit int) ([]models.WritebackItem, error)
}
