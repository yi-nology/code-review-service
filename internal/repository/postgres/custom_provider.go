package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/yi-nology/code-review-service/internal/domain/llmprovider"
	"github.com/yi-nology/code-review-service/internal/repository/postgres/models"

	gormLib "gorm.io/gorm"
)

type customProviderRepo struct {
	db *gormLib.DB
}

func NewCustomProviderRepo(db *gormLib.DB) *customProviderRepo {
	return &customProviderRepo{db: db}
}

func (r *customProviderRepo) List(ctx context.Context) ([]llmprovider.Custom, error) {
	var items []models.CustomProvider
	if err := r.db.WithContext(ctx).Order("name ASC").Find(&items).Error; err != nil {
		return nil, err
	}
	out := make([]llmprovider.Custom, len(items))
	for i, m := range items {
		out[i] = customProviderToDomain(m)
	}
	return out, nil
}

func (r *customProviderRepo) GetByID(ctx context.Context, id uint) (*llmprovider.Custom, error) {
	var m models.CustomProvider
	if err := r.db.WithContext(ctx).First(&m, id).Error; err != nil {
		return nil, err
	}
	d := customProviderToDomain(m)
	return &d, nil
}

func (r *customProviderRepo) GetByName(ctx context.Context, name string) (*llmprovider.Custom, error) {
	var m models.CustomProvider
	if err := r.db.WithContext(ctx).Where("name = ?", name).First(&m).Error; err != nil {
		return nil, err
	}
	d := customProviderToDomain(m)
	return &d, nil
}

func (r *customProviderRepo) Create(ctx context.Context, p *llmprovider.Custom) error {
	m := customProviderToModel(p)
	// Unscoped 查询含软删除记录，避免"删除后再创建同名"触发 unique index 冲突。
	var existing models.CustomProvider
	err := r.db.WithContext(ctx).Unscoped().Where("name = ?", m.Name).First(&existing).Error
	if err == nil {
		if existing.DeletedAt.Valid {
			m.ID = existing.ID
			return r.db.WithContext(ctx).Unscoped().Save(&m).Error
		}
		return fmt.Errorf("provider 名称 %s 已存在", m.Name)
	}
	if !errors.Is(err, gormLib.ErrRecordNotFound) {
		return err
	}
	return r.db.WithContext(ctx).Create(&m).Error
}

func (r *customProviderRepo) Update(ctx context.Context, p *llmprovider.Custom) error {
	m := customProviderToModel(p)
	return r.db.WithContext(ctx).Save(&m).Error
}

func (r *customProviderRepo) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&models.CustomProvider{}, id).Error
}
