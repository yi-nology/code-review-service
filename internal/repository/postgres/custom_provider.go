package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/yi-nology/code-review-service/internal/repository/postgres/models"

	gormLib "gorm.io/gorm"
)

type customProviderRepo struct {
	db *gormLib.DB
}

func NewCustomProviderRepo(db *gormLib.DB) *customProviderRepo {
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
	// Unscoped 查询含软删除记录，避免"删除后再创建同名"触发 unique index 冲突。
	var existing models.CustomProvider
	err := r.db.WithContext(ctx).Unscoped().Where("name = ?", p.Name).First(&existing).Error
	if err == nil {
		if existing.DeletedAt.Valid {
			p.ID = existing.ID
			return r.db.WithContext(ctx).Unscoped().Save(p).Error
		}
		return fmt.Errorf("provider 名称 %s 已存在", p.Name)
	}
	if !errors.Is(err, gormLib.ErrRecordNotFound) {
		return err
	}
	return r.db.WithContext(ctx).Create(p).Error
}

func (r *customProviderRepo) Update(ctx context.Context, p *models.CustomProvider) error {
	return r.db.WithContext(ctx).Save(p).Error
}

func (r *customProviderRepo) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&models.CustomProvider{}, id).Error
}
