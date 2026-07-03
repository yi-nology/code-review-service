package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/yi-nology/code-review-service/internal/domain/reviewer"
	"github.com/yi-nology/code-review-service/internal/repository/postgres/models"

	gormLib "gorm.io/gorm"
)

type ReviewerAgentRepo struct {
	db *gormLib.DB
}

func NewReviewerAgentRepo(db *gormLib.DB) *ReviewerAgentRepo {
	return &ReviewerAgentRepo{db: db}
}

func (r *ReviewerAgentRepo) List(ctx context.Context) ([]reviewer.Agent, error) {
	var agents []models.ReviewerAgent
	if err := r.db.WithContext(ctx).Order("priority ASC, id ASC").Find(&agents).Error; err != nil {
		return nil, err
	}
	out := make([]reviewer.Agent, len(agents))
	for i, m := range agents {
		out[i] = reviewerToDomain(m)
	}
	return out, nil
}

func (r *ReviewerAgentRepo) GetByID(ctx context.Context, id uint) (*reviewer.Agent, error) {
	var m models.ReviewerAgent
	if err := r.db.WithContext(ctx).First(&m, id).Error; err != nil {
		return nil, err
	}
	d := reviewerToDomain(m)
	return &d, nil
}

func (r *ReviewerAgentRepo) GetByName(ctx context.Context, name string) (*reviewer.Agent, error) {
	var m models.ReviewerAgent
	if err := r.db.WithContext(ctx).Where("name = ?", name).First(&m).Error; err != nil {
		return nil, err
	}
	d := reviewerToDomain(m)
	return &d, nil
}

func (r *ReviewerAgentRepo) Create(ctx context.Context, agent *reviewer.Agent) error {
	m := reviewerToModel(agent)
	// Unscoped 查询含软删除记录，避免"删除后再创建同名"触发 unique index 冲突。
	var existing models.ReviewerAgent
	err := r.db.WithContext(ctx).Unscoped().Where("name = ?", m.Name).First(&existing).Error
	if err == nil {
		if existing.DeletedAt.Valid {
			m.ID = existing.ID
			return r.db.WithContext(ctx).Unscoped().Save(&m).Error
		}
		return fmt.Errorf("agent 名称 %s 已存在", m.Name)
	}
	if !errors.Is(err, gormLib.ErrRecordNotFound) {
		return err
	}
	return r.db.WithContext(ctx).Create(&m).Error
}

func (r *ReviewerAgentRepo) Update(ctx context.Context, agent *reviewer.Agent) error {
	m := reviewerToModel(agent)
	return r.db.WithContext(ctx).Save(&m).Error
}

func (r *ReviewerAgentRepo) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&models.ReviewerAgent{}, id).Error
}

func (r *ReviewerAgentRepo) ListEnabled(ctx context.Context) ([]reviewer.Agent, error) {
	var agents []models.ReviewerAgent
	if err := r.db.WithContext(ctx).Where("enabled = ?", true).Order("priority ASC, id ASC").Find(&agents).Error; err != nil {
		return nil, err
	}
	out := make([]reviewer.Agent, len(agents))
	for i, m := range agents {
		out[i] = reviewerToDomain(m)
	}
	return out, nil
}
