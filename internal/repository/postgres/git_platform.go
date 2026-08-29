package postgres

import (
	"context"

	"github.com/yi-nology/code-review-service/internal/domain/platform"
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

func (r *GitPlatformRepo) List(ctx context.Context) ([]platform.Platform, error) {
	var list []models.GitPlatform
	if err := r.db.WithContext(ctx).Order("provider").Find(&list).Error; err != nil {
		return nil, err
	}
	out := make([]platform.Platform, len(list))
	for i, m := range list {
		out[i] = platformToDomain(m)
	}
	return out, nil
}

func (r *GitPlatformRepo) Get(ctx context.Context, provider, installationID string) (*platform.Platform, error) {
	var m models.GitPlatform
	err := r.db.WithContext(ctx).Where("provider = ? AND installation_id = ?", provider, installationID).First(&m).Error
	if err != nil {
		return nil, wrapNotFound(err)
	}
	d := platformToDomain(m)
	return &d, nil
}

func (r *GitPlatformRepo) Save(ctx context.Context, p *platform.Platform) error {
	m := platformToModel(p)
	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "provider"}, {Name: "installation_id"}},
			DoUpdates: clause.AssignmentColumns([]string{"base_url", "token", "webhook_secret", "enabled", "skip_tls", "updated_at", "deleted_at"}),
		}).
		Create(&m).Error
}

// Delete 按 (provider, installation_id) 精确软删，避免多 installation 场景下误删同 provider 的其它 installation。
func (r *GitPlatformRepo) Delete(ctx context.Context, provider, installationID string) error {
	return r.db.WithContext(ctx).Where("provider = ? AND installation_id = ?", provider, installationID).Delete(&models.GitPlatform{}).Error
}

func (r *GitPlatformRepo) ListEnabled(ctx context.Context) ([]platform.Platform, error) {
	var list []models.GitPlatform
	if err := r.db.WithContext(ctx).Where("enabled = ?", true).Order("provider").Find(&list).Error; err != nil {
		return nil, err
	}
	out := make([]platform.Platform, len(list))
	for i, m := range list {
		out[i] = platformToDomain(m)
	}
	return out, nil
}
