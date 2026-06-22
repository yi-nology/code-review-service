package postgres

import (
	"context"
	"errors"

	"github.com/yi-nology/code-review-service/internal/repository/postgres/models"

	gormLib "gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type GitPlatformRepo struct {
	db *gormLib.DB
}

func NewGitPlatformRepo(db *gormLib.DB) *GitPlatformRepo {
	return &GitPlatformRepo{db: db}
}

func (r *GitPlatformRepo) List(ctx context.Context) ([]models.GitPlatform, error) {
	var list []models.GitPlatform
	err := r.db.WithContext(ctx).Order("provider").Find(&list).Error
	return list, err
}

func (r *GitPlatformRepo) Get(ctx context.Context, provider string) (*models.GitPlatform, error) {
	var p models.GitPlatform
	err := r.db.WithContext(ctx).Where("provider = ?", provider).First(&p).Error
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *GitPlatformRepo) Save(ctx context.Context, p *models.GitPlatform) error {
	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "provider"}, {Name: "installation_id"}},
			DoUpdates: clause.AssignmentColumns([]string{"base_url", "token", "webhook_secret", "enabled", "updated_at", "deleted_at"}),
		}).
		Create(p).Error
}

func (r *GitPlatformRepo) Delete(ctx context.Context, provider string) error {
	return r.db.WithContext(ctx).Where("provider = ?", provider).Delete(&models.GitPlatform{}).Error
}

func (r *GitPlatformRepo) ListEnabled(ctx context.Context) ([]models.GitPlatform, error) {
	var list []models.GitPlatform
	err := r.db.WithContext(ctx).Where("enabled = ?", true).Order("provider").Find(&list).Error
	return list, err
}

func (r *GitPlatformRepo) GetByProviderAndInstallation(ctx context.Context, provider, installationID string) (*models.GitPlatform, error) {
	var p models.GitPlatform
	err := r.db.WithContext(ctx).Where("provider = ? AND installation_id = ?", provider, installationID).First(&p).Error
	if err != nil {
		if errors.Is(err, gormLib.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &p, nil
}
