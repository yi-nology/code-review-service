package postgres

import (
	"context"
	"errors"

	"github.com/yi-nology/code-review-service/internal/domain/rules"
	"github.com/yi-nology/code-review-service/internal/repository/postgres/models"

	gormLib "gorm.io/gorm"
)

type GlobalRulesRepo struct {
	db *gormLib.DB
}

func NewGlobalRulesRepo(db *gormLib.DB) *GlobalRulesRepo {
	return &GlobalRulesRepo{db: db}
}

func (r *GlobalRulesRepo) Get(ctx context.Context) (*rules.Global, error) {
	var m models.GlobalRules
	err := r.db.WithContext(ctx).
		Order("id DESC").
		First(&m).Error
	if err != nil {
		if errors.Is(err, gormLib.ErrRecordNotFound) {
			return &rules.Global{}, nil
		}
		return nil, err
	}
	d := globalRulesToDomain(m)
	return &d, nil
}

func (r *GlobalRulesRepo) Save(ctx context.Context, g *rules.Global) error {
	m := globalRulesToModel(g)
	return r.db.WithContext(ctx).Transaction(func(tx *gormLib.DB) error {
		var existing models.GlobalRules
		err := tx.Order("id DESC").First(&existing).Error
		if err != nil && !errors.Is(err, gormLib.ErrRecordNotFound) {
			return err
		}
		if existing.ID > 0 {
			m.ID = existing.ID
			return tx.Save(&m).Error
		}
		return tx.Create(&m).Error
	})
}
