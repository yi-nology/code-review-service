package repository

import (
	"context"

	"github.com/yi-nology/code-review-service/internal/domain/reviewer"
)

// ReviewerAgentRepository 已迁移为 domain 类型（#6）：不再依赖 postgres/models。
type ReviewerAgentRepository interface {
	List(ctx context.Context) ([]reviewer.Agent, error)
	GetByID(ctx context.Context, id uint) (*reviewer.Agent, error)
	GetByName(ctx context.Context, name string) (*reviewer.Agent, error)
	Create(ctx context.Context, agent *reviewer.Agent) error
	Update(ctx context.Context, agent *reviewer.Agent) error
	Delete(ctx context.Context, id uint) error
	ListEnabled(ctx context.Context) ([]reviewer.Agent, error)
}

// ProjectAgentRepository 已迁移为 domain 类型（#6）。
type ProjectAgentRepository interface {
	GetByProject(ctx context.Context, provider, repo string) ([]reviewer.ProjectAssignment, error)
	GetEnabledByProject(ctx context.Context, provider, repo string) ([]reviewer.ProjectAssignment, error)
	Upsert(ctx context.Context, pa *reviewer.ProjectAssignment) error
	Delete(ctx context.Context, provider, repo string, agentID uint) error
	DeleteByProject(ctx context.Context, provider, repo string) error
	SetEnabled(ctx context.Context, provider, repo string, agentID uint, enabled bool) error
}
