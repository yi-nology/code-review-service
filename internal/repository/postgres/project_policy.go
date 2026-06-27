package postgres

import (
	"context"
	"errors"

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

func (r *ProjectPolicyRepo) List(ctx context.Context) ([]models.ProjectPolicy, error) {
	var policies []models.ProjectPolicy
	err := r.db.WithContext(ctx).
		Order("provider, repo").
		Find(&policies).Error
	return policies, err
}

func (r *ProjectPolicyRepo) GetByProviderRepo(ctx context.Context, provider, repo string) (*models.ProjectPolicy, error) {
	var policy models.ProjectPolicy
	err := r.db.WithContext(ctx).
		Where("provider = ? AND repo = ?", provider, repo).
		First(&policy).Error
	if err != nil {
		if errors.Is(err, gormLib.ErrRecordNotFound) {
			return nil, repoerr.ErrNotFound
		}
		return nil, err
	}
	return &policy, nil
}

func (r *ProjectPolicyRepo) Save(ctx context.Context, policy *models.ProjectPolicy) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gormLib.DB) error {
		var existing models.ProjectPolicy
		err := tx.Unscoped().
			Where("provider = ? AND repo = ?", policy.Provider, policy.Repo).
			First(&existing).Error
		if errors.Is(err, gormLib.ErrRecordNotFound) {
			return tx.Create(policy).Error
		}
		if err != nil {
			return err
		}
		policy.ID = existing.ID
		return tx.Unscoped().Save(policy).Error
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
