package postgres

import (
	"context"
	"errors"

	"github.com/yi-nology/code-review-service/internal/domain/projectpolicy"
	"github.com/yi-nology/code-review-service/internal/repository/postgres/models"
	"github.com/yi-nology/code-review-service/internal/repository/repoerr"

	gormLib "gorm.io/gorm"
)

type ProjectPolicyRepo struct {
	db *gormLib.DB
}

func NewProjectPolicyRepo(db *gormLib.DB) *ProjectPolicyRepo {
	return &ProjectPolicyRepo{db: db}
}

func (r *ProjectPolicyRepo) List(ctx context.Context) ([]projectpolicy.ProjectPolicy, error) {
	var policies []models.ProjectPolicy
	if err := r.db.WithContext(ctx).
		Order("provider, repo").
		Find(&policies).Error; err != nil {
		return nil, err
	}
	out := make([]projectpolicy.ProjectPolicy, len(policies))
	for i, m := range policies {
		out[i] = projectPolicyToDomain(m)
	}
	return out, nil
}

func (r *ProjectPolicyRepo) GetByProviderRepo(ctx context.Context, provider, repo string) (*projectpolicy.ProjectPolicy, error) {
	var m models.ProjectPolicy
	err := r.db.WithContext(ctx).
		Where("provider = ? AND repo = ?", provider, repo).
		First(&m).Error
	if err != nil {
		if errors.Is(err, gormLib.ErrRecordNotFound) {
			return nil, repoerr.ErrNotFound
		}
		return nil, err
	}
	d := projectPolicyToDomain(m)
	return &d, nil
}

func (r *ProjectPolicyRepo) Save(ctx context.Context, policy *projectpolicy.ProjectPolicy) error {
	m := projectPolicyToModel(policy)
	return r.db.WithContext(ctx).Transaction(func(tx *gormLib.DB) error {
		var existing models.ProjectPolicy
		err := tx.Unscoped().
			Where("provider = ? AND repo = ?", m.Provider, m.Repo).
			First(&existing).Error
		if errors.Is(err, gormLib.ErrRecordNotFound) {
			return tx.Create(&m).Error
		}
		if err != nil {
			return err
		}
		m.ID = existing.ID
		return tx.Unscoped().Save(&m).Error
	})
}

func (r *ProjectPolicyRepo) Delete(ctx context.Context, provider, repo string) error {
	result := r.db.WithContext(ctx).
		Where("provider = ? AND repo = ?", provider, repo).
		Delete(&models.ProjectPolicy{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gormLib.ErrRecordNotFound
	}
	return nil
}
