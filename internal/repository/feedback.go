package repository

import (
	"context"

	"github.com/yi-nology/code-review-service/internal/domain/feedback"
)

// FindingFeedbackRepository 已迁移为 domain 类型（#6）：不再依赖 postgres/models。
type FindingFeedbackRepository interface {
	Upsert(ctx context.Context, item *feedback.Feedback) error
	ListByReview(ctx context.Context, reviewID string) ([]feedback.Feedback, error)
}
