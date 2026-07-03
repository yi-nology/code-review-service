package repository

import (
	"context"

	"github.com/yi-nology/code-review-service/internal/domain/ocrasset"
	"github.com/yi-nology/code-review-service/internal/domain/ocrsettings"
	"github.com/yi-nology/code-review-service/internal/domain/projectpolicy"
	"github.com/yi-nology/code-review-service/internal/domain/rules"
)

type OCRSettingsRepository interface {
	Get(ctx context.Context) (*ocrsettings.Settings, error)
	Save(ctx context.Context, settings *ocrsettings.Settings) error
}

type ProjectPolicyRepository interface {
	List(ctx context.Context) ([]projectpolicy.ProjectPolicy, error)
	GetByProviderRepo(ctx context.Context, provider, repo string) (*projectpolicy.ProjectPolicy, error)
	Save(ctx context.Context, policy *projectpolicy.ProjectPolicy) error
	Delete(ctx context.Context, provider, repo string) error
}

type GlobalRulesRepository interface {
	Get(ctx context.Context) (*rules.Global, error)
	Save(ctx context.Context, rules *rules.Global) error
}

type OCRRulePackageRepository interface {
	List(ctx context.Context) ([]ocrasset.RulePackage, error)
	GetByID(ctx context.Context, id uint) (*ocrasset.RulePackage, error)
	Save(ctx context.Context, pkg *ocrasset.RulePackage) error
	Delete(ctx context.Context, id uint) error
	ListVersions(ctx context.Context, id uint) ([]ocrasset.RulePackageVersion, error)
	RestoreVersion(ctx context.Context, id uint, version int) (*ocrasset.RulePackage, error)
}

type OCRToolPackageRepository interface {
	List(ctx context.Context) ([]ocrasset.ToolPackage, error)
	GetByID(ctx context.Context, id uint) (*ocrasset.ToolPackage, error)
	Save(ctx context.Context, pkg *ocrasset.ToolPackage) error
	Delete(ctx context.Context, id uint) error
	ListVersions(ctx context.Context, id uint) ([]ocrasset.ToolPackageVersion, error)
	RestoreVersion(ctx context.Context, id uint, version int) (*ocrasset.ToolPackage, error)
}
