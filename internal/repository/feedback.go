package repository

import (
	"context"

	"github.com/yi-nology/code-review-service/internal/repository/postgres/models"
)

type FindingFeedbackRepository interface {
	Upsert(ctx context.Context, item *models.FindingFeedback) error
	ListByReview(ctx context.Context, reviewID string) ([]models.FindingFeedback, error)
}
