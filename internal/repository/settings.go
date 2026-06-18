package repository

import (
	"context"

	"github.com/yi-nology/code-review-service/internal/repository/postgres/models"
)

type OCRSettingsRepository interface {
	Get(ctx context.Context) (*models.OCRSettings, error)
	Save(ctx context.Context, settings *models.OCRSettings) error
}

type ProjectPolicyRepository interface {
	List(ctx context.Context) ([]models.ProjectPolicy, error)
	GetByProviderRepo(ctx context.Context, provider, repo string) (*models.ProjectPolicy, error)
	Save(ctx context.Context, policy *models.ProjectPolicy) error
	Delete(ctx context.Context, provider, repo string) error
}

type GlobalRulesRepository interface {
	Get(ctx context.Context) (*models.GlobalRules, error)
	Save(ctx context.Context, rules *models.GlobalRules) error
}
