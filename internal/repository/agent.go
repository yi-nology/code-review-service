package repository

import (
	"context"

	"github.com/yi-nology/code-review-service/internal/repository/postgres/models"
)

type ReviewerAgentRepository interface {
	List(ctx context.Context) ([]models.ReviewerAgent, error)
	GetByID(ctx context.Context, id uint) (*models.ReviewerAgent, error)
	GetByName(ctx context.Context, name string) (*models.ReviewerAgent, error)
	Create(ctx context.Context, agent *models.ReviewerAgent) error
	Update(ctx context.Context, agent *models.ReviewerAgent) error
	Delete(ctx context.Context, id uint) error
	ListEnabled(ctx context.Context) ([]models.ReviewerAgent, error)
}

type ProjectAgentRepository interface {
	GetByProject(ctx context.Context, provider, repo string) ([]models.ProjectAgent, error)
	GetEnabledByProject(ctx context.Context, provider, repo string) ([]models.ProjectAgent, error)
	Upsert(ctx context.Context, pa *models.ProjectAgent) error
	Delete(ctx context.Context, provider, repo string, agentID uint) error
	DeleteByProject(ctx context.Context, provider, repo string) error
	SetEnabled(ctx context.Context, provider, repo string, agentID uint, enabled bool) error
}
