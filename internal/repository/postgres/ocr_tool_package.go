package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/yi-nology/code-review-service/internal/domain/ocrasset"
	"github.com/yi-nology/code-review-service/internal/repository/postgres/models"

	"gorm.io/datatypes"
	gormLib "gorm.io/gorm"
)

type OCRToolPackageRepo struct {
	db *gormLib.DB
}

func NewOCRToolPackageRepo(db *gormLib.DB) *OCRToolPackageRepo {
	return &OCRToolPackageRepo{db: db}
}

func (r *OCRToolPackageRepo) List(ctx context.Context) ([]ocrasset.ToolPackage, error) {
	var items []models.OCRToolPackage
	if err := r.db.WithContext(ctx).Order("built_in DESC, name ASC").Find(&items).Error; err != nil {
		return nil, err
	}
	out := make([]ocrasset.ToolPackage, len(items))
	for i, m := range items {
		out[i] = toolPackageToDomain(m)
	}
	return out, nil
}

func (r *OCRToolPackageRepo) GetByID(ctx context.Context, id uint) (*ocrasset.ToolPackage, error) {
	var m models.OCRToolPackage
	if err := r.db.WithContext(ctx).First(&m, id).Error; err != nil {
		return nil, err
	}
	d := toolPackageToDomain(m)
	return &d, nil
}

func (r *OCRToolPackageRepo) Save(ctx context.Context, pkg *ocrasset.ToolPackage) error {
	m := toolPackageToModel(pkg)
	return r.db.WithContext(ctx).Transaction(func(tx *gormLib.DB) error {
		if m.ID != 0 {
			var existing models.OCRToolPackage
			if err := tx.First(&existing, m.ID).Error; err == nil {
				if err := createToolPackageVersion(ctx, tx, &existing); err != nil {
					return err
				}
			} else if !errors.Is(err, gormLib.ErrRecordNotFound) {
				return err
			}
		}
		return tx.Save(&m).Error
	})
}

func (r *OCRToolPackageRepo) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&models.OCRToolPackage{}, id).Error
}

func (r *OCRToolPackageRepo) ListVersions(ctx context.Context, id uint) ([]ocrasset.ToolPackageVersion, error) {
	var items []models.OCRToolPackageVersion
	if err := r.db.WithContext(ctx).Where("package_id = ?", id).Order("version DESC").Find(&items).Error; err != nil {
		return nil, err
	}
	out := make([]ocrasset.ToolPackageVersion, len(items))
	for i, m := range items {
		out[i] = toolPackageVersionToDomain(m)
	}
	return out, nil
}

func (r *OCRToolPackageRepo) RestoreVersion(ctx context.Context, id uint, version int) (*ocrasset.ToolPackage, error) {
	var restored models.OCRToolPackage
	err := r.db.WithContext(ctx).Transaction(func(tx *gormLib.DB) error {
		var current models.OCRToolPackage
		if err := tx.First(&current, id).Error; err != nil {
			return err
		}
		var item models.OCRToolPackageVersion
		if err := tx.Where("package_id = ? AND version = ?", id, version).First(&item).Error; err != nil {
			return err
		}
		if err := createToolPackageVersion(ctx, tx, &current); err != nil {
			return err
		}
		if err := json.Unmarshal(item.Snapshot, &restored); err != nil {
			return fmt.Errorf("decode tool package snapshot: %w", err)
		}
		restored.ID = id
		restored.CreatedAt = current.CreatedAt
		restored.UpdatedAt = current.UpdatedAt
		return tx.Save(&restored).Error
	})
	if err != nil {
		return nil, err
	}
	d := toolPackageToDomain(restored)
	return &d, nil
}

func createToolPackageVersion(ctx context.Context, tx *gormLib.DB, pkg *models.OCRToolPackage) error {
	var maxVersion int
	if err := tx.WithContext(ctx).Model(&models.OCRToolPackageVersion{}).
		Where("package_id = ?", pkg.ID).
		Select("COALESCE(MAX(version), 0)").
		Scan(&maxVersion).Error; err != nil {
		return err
	}
	data, err := json.Marshal(pkg)
	if err != nil {
		return err
	}
	return tx.WithContext(ctx).Create(&models.OCRToolPackageVersion{
		PackageID: pkg.ID,
		Version:   maxVersion + 1,
		Snapshot:  datatypes.JSON(data),
	}).Error
}
