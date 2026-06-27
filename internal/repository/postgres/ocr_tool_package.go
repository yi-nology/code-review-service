package postgres

import (
	"context"

	"github.com/yi-nology/code-review-service/internal/repository/postgres/models"

	gormLib "gorm.io/gorm"
)

type OCRToolPackageRepo struct {
	db *gormLib.DB
}

func NewOCRToolPackageRepo(db *gormLib.DB) *OCRToolPackageRepo {
	return &OCRToolPackageRepo{db: db}
}

func (r *OCRToolPackageRepo) List(ctx context.Context) ([]models.OCRToolPackage, error) {
	var items []models.OCRToolPackage
	err := r.db.WithContext(ctx).Order("built_in DESC, name ASC").Find(&items).Error
	return items, err
}

func (r *OCRToolPackageRepo) GetByID(ctx context.Context, id uint) (*models.OCRToolPackage, error) {
	var item models.OCRToolPackage
	if err := r.db.WithContext(ctx).First(&item, id).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *OCRToolPackageRepo) Save(ctx context.Context, pkg *models.OCRToolPackage) error {
	return r.db.WithContext(ctx).Save(pkg).Error
}

func (r *OCRToolPackageRepo) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&models.OCRToolPackage{}, id).Error
}
