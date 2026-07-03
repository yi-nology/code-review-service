package repository

import (
	"context"

	"github.com/yi-nology/code-review-service/internal/domain/platform"
	"github.com/yi-nology/code-review-service/internal/domain/revision"
)

// GitPlatformRepository 已迁移为 domain 类型（#6）：不再依赖 postgres/models。
type GitPlatformRepository interface {
	List(ctx context.Context) ([]platform.Platform, error)
	Get(ctx context.Context, provider, installationID string) (*platform.Platform, error)
	Save(ctx context.Context, p *platform.Platform) error
	Delete(ctx context.Context, provider, installationID string) error
	ListEnabled(ctx context.Context) ([]platform.Platform, error)
	GetByProviderAndInstallation(ctx context.Context, provider, installationID string) (*platform.Platform, error)
}

// ChangeRevisionRepository 已迁移为 domain 类型（#6）。
type ChangeRevisionRepository interface {
	Upsert(ctx context.Context, cr *revision.Change) error
	GetByChange(ctx context.Context, provider, installationID, repositoryID string, changeNumber int) (*revision.Change, error)
	GetCurrentHeadSHA(ctx context.Context, provider, installationID, repositoryID string, changeNumber int) (string, error)
}
