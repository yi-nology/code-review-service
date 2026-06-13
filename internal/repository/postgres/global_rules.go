package postgres

import (
	"context"

	"review-service/internal/repository/postgres/models"

	gormLib "gorm.io/gorm"
)

type GlobalRulesRepo struct {
	db *gormLib.DB
}

func NewGlobalRulesRepo(db *gormLib.DB) *GlobalRulesRepo {
	return &GlobalRulesRepo{db: db}
}

func (r *GlobalRulesRepo) Get(ctx context.Context) (*models.GlobalRules, error) {
	var rules models.GlobalRules
	err := r.db.WithContext(ctx).
		Order("id DESC").
		First(&rules).Error
	if err != nil {
		if err == gormLib.ErrRecordNotFound {
			return &models.GlobalRules{}, nil
		}
		return nil, err
	}
	return &rules, nil
}

func (r *GlobalRulesRepo) Save(ctx context.Context, rules *models.GlobalRules) error {
	var existing models.GlobalRules
	err := r.db.WithContext(ctx).
		Order("id DESC").
		First(&existing).Error
	if err != nil && err != gormLib.ErrRecordNotFound {
		return err
	}
	if existing.ID > 0 {
		rules.ID = existing.ID
		return r.db.WithContext(ctx).Save(rules).Error
	}
	return r.db.WithContext(ctx).Create(rules).Error
}
