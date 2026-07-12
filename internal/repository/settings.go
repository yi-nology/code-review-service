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
	// GetOrCreateDefault 读取项目策略；不存在时自动创建一条默认记录（FailOn=major）并返回。
	// 并发安全（ON CONFLICT DO NOTHING）。创建失败（非唯一冲突）时不阻断，返回默认值。
	GetOrCreateDefault(ctx context.Context, provider, repo string) (projectpolicy.ProjectPolicy, error)
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
	// InsertIfAbsent 插入 pkg（要求 ID=0）；当 (name) 唯一约束冲突时忽略（返回已存在记录的 ID，不报错）。
	// 适用于「启动期迁移/seed」等 k8s 多副本并发下需要 race-safe 幂等插入的场景。
	// pkg.ID 必填 0，传入非零返回错误。
	InsertIfAbsent(ctx context.Context, pkg *ocrasset.RulePackage) (uint, error)
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
