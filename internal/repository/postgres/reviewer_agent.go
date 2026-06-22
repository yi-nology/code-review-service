package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/yi-nology/code-review-service/internal/repository/postgres/models"

	gormLib "gorm.io/gorm"
)

type ReviewerAgentRepo struct {
	db *gormLib.DB
}

func NewReviewerAgentRepo(db *gormLib.DB) *ReviewerAgentRepo {
	return &ReviewerAgentRepo{db: db}
}

func (r *ReviewerAgentRepo) List(ctx context.Context) ([]models.ReviewerAgent, error) {
	var agents []models.ReviewerAgent
	err := r.db.WithContext(ctx).Order("priority ASC, id ASC").Find(&agents).Error
	return agents, err
}

func (r *ReviewerAgentRepo) GetByID(ctx context.Context, id uint) (*models.ReviewerAgent, error) {
	var agent models.ReviewerAgent
	err := r.db.WithContext(ctx).First(&agent, id).Error
	if err != nil {
		return nil, err
	}
	return &agent, nil
}

func (r *ReviewerAgentRepo) GetByName(ctx context.Context, name string) (*models.ReviewerAgent, error) {
	var agent models.ReviewerAgent
	err := r.db.WithContext(ctx).Where("name = ?", name).First(&agent).Error
	if err != nil {
		return nil, err
	}
	return &agent, nil
}

func (r *ReviewerAgentRepo) Create(ctx context.Context, agent *models.ReviewerAgent) error {
	// Unscoped 查询含软删除记录，避免"删除后再创建同名"触发 unique index 冲突。
	var existing models.ReviewerAgent
	err := r.db.WithContext(ctx).Unscoped().Where("name = ?", agent.Name).First(&existing).Error
	if err == nil {
		if existing.DeletedAt.Valid {
			agent.ID = existing.ID
			return r.db.WithContext(ctx).Unscoped().Save(agent).Error
		}
		return fmt.Errorf("agent 名称 %s 已存在", agent.Name)
	}
	if !errors.Is(err, gormLib.ErrRecordNotFound) {
		return err
	}
	return r.db.WithContext(ctx).Create(agent).Error
}

func (r *ReviewerAgentRepo) Update(ctx context.Context, agent *models.ReviewerAgent) error {
	return r.db.WithContext(ctx).Save(agent).Error
}

func (r *ReviewerAgentRepo) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&models.ReviewerAgent{}, id).Error
}

func (r *ReviewerAgentRepo) ListEnabled(ctx context.Context) ([]models.ReviewerAgent, error) {
	var agents []models.ReviewerAgent
	err := r.db.WithContext(ctx).Where("enabled = ?", true).Order("priority ASC, id ASC").Find(&agents).Error
	return agents, err
}
