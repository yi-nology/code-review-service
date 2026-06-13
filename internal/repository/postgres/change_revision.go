package postgres

import (
	"context"

	"github.com/yi-nology/code-review-service/internal/repository/postgres/models"

	gormLib "gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type ChangeRevisionRepo struct {
	db *gormLib.DB
}

func NewChangeRevisionRepo(db *gormLib.DB) *ChangeRevisionRepo {
	return &ChangeRevisionRepo{db: db}
}

func (r *ChangeRevisionRepo) Upsert(ctx context.Context, cr *models.ChangeRevision) error {
	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "provider"}, {Name: "installation_id"}, {Name: "repository_id"}, {Name: "change_number"}, {Name: "head_sha"}},
			DoUpdates: clause.AssignmentColumns([]string{"base_sha", "start_sha", "latest_review_id"}),
		}).
		Create(cr).Error
}

func (r *ChangeRevisionRepo) GetByChange(ctx context.Context, provider, installationID, repositoryID string, changeNumber int) (*models.ChangeRevision, error) {
	var cr models.ChangeRevision
	err := r.db.WithContext(ctx).
		Where("provider = ? AND installation_id = ? AND repository_id = ? AND change_number = ?",
			provider, installationID, repositoryID, changeNumber).
		Order("created_at DESC").
		First(&cr).Error
	if err != nil {
		if err == gormLib.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &cr, nil
}

func (r *ChangeRevisionRepo) GetCurrentHeadSHA(ctx context.Context, provider, installationID, repositoryID string, changeNumber int) (string, error) {
	var cr models.ChangeRevision
	err := r.db.WithContext(ctx).
		Where("provider = ? AND installation_id = ? AND repository_id = ? AND change_number = ?",
			provider, installationID, repositoryID, changeNumber).
		Order("created_at DESC").
		Select("head_sha").
		First(&cr).Error
	if err != nil {
		if err == gormLib.ErrRecordNotFound {
			return "", nil
		}
		return "", err
	}
	return cr.HeadSHA, nil
}
