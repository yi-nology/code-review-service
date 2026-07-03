package repository

import (
	"context"

	"github.com/yi-nology/code-review-service/internal/domain/rules"
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
	Get(ctx context.Context) (*rules.Global, error)
	Save(ctx context.Context, rules *rules.Global) error
}

type OCRRulePackageRepository interface {
	List(ctx context.Context) ([]models.OCRRulePackage, error)
	GetByID(ctx context.Context, id uint) (*models.OCRRulePackage, error)
	Save(ctx context.Context, pkg *models.OCRRulePackage) error
	Delete(ctx context.Context, id uint) error
	ListVersions(ctx context.Context, id uint) ([]models.OCRRulePackageVersion, error)
	RestoreVersion(ctx context.Context, id uint, version int) (*models.OCRRulePackage, error)
}

type OCRToolPackageRepository interface {
	List(ctx context.Context) ([]models.OCRToolPackage, error)
	GetByID(ctx context.Context, id uint) (*models.OCRToolPackage, error)
	Save(ctx context.Context, pkg *models.OCRToolPackage) error
	Delete(ctx context.Context, id uint) error
	ListVersions(ctx context.Context, id uint) ([]models.OCRToolPackageVersion, error)
	RestoreVersion(ctx context.Context, id uint, version int) (*models.OCRToolPackage, error)
}
