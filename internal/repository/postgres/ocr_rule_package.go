package postgres

import (
	"context"

	"github.com/yi-nology/code-review-service/internal/repository/postgres/models"

	gormLib "gorm.io/gorm"
)

type OCRRulePackageRepo struct {
	db *gormLib.DB
}

func NewOCRRulePackageRepo(db *gormLib.DB) *OCRRulePackageRepo {
	return &OCRRulePackageRepo{db: db}
}

func (r *OCRRulePackageRepo) List(ctx context.Context) ([]models.OCRRulePackage, error) {
	var items []models.OCRRulePackage
	err := r.db.WithContext(ctx).Order("built_in DESC, name ASC").Find(&items).Error
	return items, err
}

func (r *OCRRulePackageRepo) GetByID(ctx context.Context, id uint) (*models.OCRRulePackage, error) {
	var item models.OCRRulePackage
	if err := r.db.WithContext(ctx).First(&item, id).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *OCRRulePackageRepo) Save(ctx context.Context, pkg *models.OCRRulePackage) error {
	return r.db.WithContext(ctx).Save(pkg).Error
}

func (r *OCRRulePackageRepo) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&models.OCRRulePackage{}, id).Error
}
