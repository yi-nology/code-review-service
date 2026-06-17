package repository

import (
	"context"

	"github.com/yi-nology/code-review-service/internal/repository/postgres/models"
)

type GitPlatformRepository interface {
	List(ctx context.Context) ([]models.GitPlatform, error)
	Get(ctx context.Context, provider string) (*models.GitPlatform, error)
	Save(ctx context.Context, p *models.GitPlatform) error
	Delete(ctx context.Context, provider string) error
	ListEnabled(ctx context.Context) ([]models.GitPlatform, error)
	GetByProviderAndInstallation(ctx context.Context, provider, installationID string) (*models.GitPlatform, error)
}

type ChangeRevisionRepository interface {
	Upsert(ctx context.Context, cr *models.ChangeRevision) error
	GetByChange(ctx context.Context, provider, installationID, repositoryID string, changeNumber int) (*models.ChangeRevision, error)
	GetCurrentHeadSHA(ctx context.Context, provider, installationID, repositoryID string, changeNumber int) (string, error)
}
