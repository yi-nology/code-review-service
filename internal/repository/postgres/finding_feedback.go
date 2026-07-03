package postgres

import (
	"context"

	"github.com/yi-nology/code-review-service/internal/domain/feedback"
	"github.com/yi-nology/code-review-service/internal/repository/postgres/models"

	gormLib "gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type FindingFeedbackRepo struct {
	db *gormLib.DB
}

func NewFindingFeedbackRepo(db *gormLib.DB) *FindingFeedbackRepo {
	return &FindingFeedbackRepo{db: db}
}

func (r *FindingFeedbackRepo) Upsert(ctx context.Context, item *feedback.Feedback) error {
	m := feedbackToModel(item)
	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "review_id"}, {Name: "finding_index"}},
			DoUpdates: clause.AssignmentColumns([]string{
				"status",
				"note",
				"updated_at",
			}),
		}).
		Create(&m).Error
}

func (r *FindingFeedbackRepo) ListByReview(ctx context.Context, reviewID string) ([]feedback.Feedback, error) {
	var items []models.FindingFeedback
	err := r.db.WithContext(ctx).
		Where("review_id = ?", reviewID).
		Order("finding_index ASC").
		Find(&items).Error
	if err != nil {
		return nil, err
	}
	out := make([]feedback.Feedback, len(items))
	for i, m := range items {
		out[i] = feedbackToDomain(m)
	}
	return out, nil
}
