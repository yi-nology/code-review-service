package postgres

import (
	"context"
	"errors"

	"github.com/yi-nology/code-review-service/internal/domain/revision"
	"github.com/yi-nology/code-review-service/internal/repository/postgres/models"
	"github.com/yi-nology/code-review-service/internal/repository/repoerr"

	gormLib "gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type ChangeRevisionRepo struct {
	db *gormLib.DB
}

func NewChangeRevisionRepo(db *gormLib.DB) *ChangeRevisionRepo {
	return &ChangeRevisionRepo{db: db}
}

func (r *ChangeRevisionRepo) Upsert(ctx context.Context, cr *revision.Change) error {
	m := revisionToModel(cr)
	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "provider"}, {Name: "installation_id"}, {Name: "repository_id"}, {Name: "change_number"}, {Name: "head_sha"}},
			DoUpdates: clause.AssignmentColumns([]string{"base_sha", "start_sha", "latest_review_id"}),
		}).
		Create(&m).Error
}

func (r *ChangeRevisionRepo) GetByChange(ctx context.Context, provider, installationID, repositoryID string, changeNumber int) (*revision.Change, error) {
	var m models.ChangeRevision
	err := r.db.WithContext(ctx).
		Where("provider = ? AND installation_id = ? AND repository_id = ? AND change_number = ?",
			provider, installationID, repositoryID, changeNumber).
		Order("created_at DESC").
		First(&m).Error
	if err != nil {
		if errors.Is(err, gormLib.ErrRecordNotFound) {
			return nil, repoerr.ErrNotFound
		}
		return nil, err
	}
	d := revisionToDomain(m)
	return &d, nil
}

func (r *ChangeRevisionRepo) GetCurrentHeadSHA(ctx context.Context, provider, installationID, repositoryID string, changeNumber int) (string, error) {
	var m models.ChangeRevision
	err := r.db.WithContext(ctx).
		Where("provider = ? AND installation_id = ? AND repository_id = ? AND change_number = ?",
			provider, installationID, repositoryID, changeNumber).
		Order("created_at DESC").
		Select("head_sha").
		First(&m).Error
	if err != nil {
		if errors.Is(err, gormLib.ErrRecordNotFound) {
			return "", nil
		}
		return "", err
	}
	return m.HeadSHA, nil
}
