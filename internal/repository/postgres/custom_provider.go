package postgres

import (
	"context"

	"github.com/yi-nology/code-review-service/internal/repository/postgres/models"

	"gorm.io/gorm"
)

type customProviderRepo struct {
	db *gorm.DB
}

func NewCustomProviderRepo(db *gorm.DB) *customProviderRepo {
	return &customProviderRepo{db: db}
}

func (r *customProviderRepo) List(ctx context.Context) ([]models.CustomProvider, error) {
	var items []models.CustomProvider
	err := r.db.WithContext(ctx).Order("name ASC").Find(&items).Error
	return items, err
}

func (r *customProviderRepo) GetByID(ctx context.Context, id uint) (*models.CustomProvider, error) {
	var item models.CustomProvider
	err := r.db.WithContext(ctx).First(&item, id).Error
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *customProviderRepo) GetByName(ctx context.Context, name string) (*models.CustomProvider, error) {
	var item models.CustomProvider
	err := r.db.WithContext(ctx).Where("name = ?", name).First(&item).Error
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *customProviderRepo) Create(ctx context.Context, p *models.CustomProvider) error {
	return r.db.WithContext(ctx).Create(p).Error
}

func (r *customProviderRepo) Update(ctx context.Context, p *models.CustomProvider) error {
	return r.db.WithContext(ctx).Save(p).Error
}

func (r *customProviderRepo) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&models.CustomProvider{}, id).Error
}
