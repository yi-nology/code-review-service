package postgres

import (
	"context"
	"errors"

	"github.com/yi-nology/code-review-service/internal/domain/reviewer"
	"github.com/yi-nology/code-review-service/internal/repository/postgres/models"

	gormLib "gorm.io/gorm"
)

type ProjectAgentRepo struct {
	db *gormLib.DB
}

func NewProjectAgentRepo(db *gormLib.DB) *ProjectAgentRepo {
	return &ProjectAgentRepo{db: db}
}

// GetByProject returns all agent associations for a project
func (r *ProjectAgentRepo) GetByProject(ctx context.Context, provider, repo string) ([]reviewer.ProjectAssignment, error) {
	var agents []models.ProjectAgent
	if err := r.db.WithContext(ctx).
		Where("provider = ? AND repo = ?", provider, repo).
		Order("priority ASC, id ASC").
		Find(&agents).Error; err != nil {
		return nil, err
	}
	out := make([]reviewer.ProjectAssignment, len(agents))
	for i, m := range agents {
		out[i] = projectAgentToDomain(m)
	}
	return out, nil
}

// GetEnabledByProject returns enabled agent associations for a project with agent details
func (r *ProjectAgentRepo) GetEnabledByProject(ctx context.Context, provider, repo string) ([]reviewer.ProjectAssignment, error) {
	var agents []models.ProjectAgent
	if err := r.db.WithContext(ctx).
		Table("project_agents").
		Select("project_agents.*, reviewer_agents.name as agent_name, reviewer_agents.display_name, reviewer_agents.background").
		Joins("LEFT JOIN reviewer_agents ON reviewer_agents.id = project_agents.agent_id").
		Where("project_agents.provider = ? AND project_agents.repo = ? AND project_agents.enabled = ? AND reviewer_agents.deleted_at IS NULL", provider, repo, true).
		Order("project_agents.priority ASC, project_agents.id ASC").
		Find(&agents).Error; err != nil {
		return nil, err
	}
	out := make([]reviewer.ProjectAssignment, len(agents))
	for i, m := range agents {
		out[i] = projectAgentToDomain(m)
	}
	return out, nil
}

// Upsert creates or updates a project agent association
func (r *ProjectAgentRepo) Upsert(ctx context.Context, pa *reviewer.ProjectAssignment) error {
	m := projectAgentToModel(pa)
	// Check if exists (including soft-deleted records)
	var existing models.ProjectAgent
	err := r.db.WithContext(ctx).
		Unscoped().
		Where("provider = ? AND repo = ? AND agent_id = ?", m.Provider, m.Repo, m.AgentID).
		First(&existing).Error

	if errors.Is(err, gormLib.ErrRecordNotFound) {
		// Create new
		return r.db.WithContext(ctx).Create(&m).Error
	}
	if err != nil {
		return err
	}

	if existing.DeletedAt.Valid {
		// Restore soft-deleted record
		return r.db.WithContext(ctx).
			Model(&existing).
			Unscoped().
			Updates(map[string]interface{}{
				"deleted_at": nil,
				"enabled":    m.Enabled,
				"priority":   m.Priority,
			}).Error
	}

	// Update existing
	return r.db.WithContext(ctx).
		Model(&existing).
		Updates(map[string]interface{}{
			"enabled":  m.Enabled,
			"priority": m.Priority,
		}).Error
}

// Delete removes a project agent association
func (r *ProjectAgentRepo) Delete(ctx context.Context, provider, repo string, agentID uint) error {
	return r.db.WithContext(ctx).
		Where("provider = ? AND repo = ? AND agent_id = ?", provider, repo, agentID).
		Delete(&models.ProjectAgent{}).Error
}

// DeleteByProject removes all agent associations for a project
func (r *ProjectAgentRepo) DeleteByProject(ctx context.Context, provider, repo string) error {
	return r.db.WithContext(ctx).
		Where("provider = ? AND repo = ?", provider, repo).
		Delete(&models.ProjectAgent{}).Error
}

// SetEnabled updates the enabled status of a project agent
func (r *ProjectAgentRepo) SetEnabled(ctx context.Context, provider, repo string, agentID uint, enabled bool) error {
	return r.db.WithContext(ctx).
		Model(&models.ProjectAgent{}).
		Where("provider = ? AND repo = ? AND agent_id = ?", provider, repo, agentID).
		Update("enabled", enabled).Error
}
