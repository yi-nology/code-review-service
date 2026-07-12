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
	"gorm.io/gorm/clause"
)

type OCRRulePackageRepo struct {
	db *gormLib.DB
}

func NewOCRRulePackageRepo(db *gormLib.DB) *OCRRulePackageRepo {
	return &OCRRulePackageRepo{db: db}
}

func (r *OCRRulePackageRepo) List(ctx context.Context) ([]ocrasset.RulePackage, error) {
	var items []models.OCRRulePackage
	if err := r.db.WithContext(ctx).Order("built_in DESC, name ASC").Find(&items).Error; err != nil {
		return nil, err
	}
	out := make([]ocrasset.RulePackage, len(items))
	for i, m := range items {
		out[i] = rulePackageToDomain(m)
	}
	return out, nil
}

func (r *OCRRulePackageRepo) GetByID(ctx context.Context, id uint) (*ocrasset.RulePackage, error) {
	var m models.OCRRulePackage
	if err := r.db.WithContext(ctx).First(&m, id).Error; err != nil {
		return nil, err
	}
	d := rulePackageToDomain(m)
	return &d, nil
}

func (r *OCRRulePackageRepo) Save(ctx context.Context, pkg *ocrasset.RulePackage) error {
	m := rulePackageToModel(pkg)
	return r.db.WithContext(ctx).Transaction(func(tx *gormLib.DB) error {
		if m.ID != 0 {
			var existing models.OCRRulePackage
			if err := tx.First(&existing, m.ID).Error; err == nil {
				if err := createRulePackageVersion(ctx, tx, &existing); err != nil {
					return err
				}
			} else if !errors.Is(err, gormLib.ErrRecordNotFound) {
				return err
			}
		}
		return tx.Save(&m).Error
	})
}

// InsertIfAbsent 插入 pkg；同名记录已存在时通过 ON CONFLICT (name) DO NOTHING 跳过，返回已存在记录的 ID。
// race-safe：k8s 多副本启动并发时只有一个能真正插入，其余拿到同一 ID 而非报错。
// pkg.ID 必须为 0；非零返回错误（语义上是 insert 不是 upsert）。
func (r *OCRRulePackageRepo) InsertIfAbsent(ctx context.Context, pkg *ocrasset.RulePackage) (uint, error) {
	m := rulePackageToModel(pkg)
	if m.ID != 0 {
		return 0, fmt.Errorf("InsertIfAbsent requires pkg.ID=0, got %d", m.ID)
	}
	// ON CONFLICT (name) DO NOTHING：被吞时 m.ID 仍为 0，需要重读拿现存的 ID。
	if err := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "name"}}, DoNothing: true}).
		Create(&m).Error; err != nil {
		return 0, fmt.Errorf("insert rule package: %w", err)
	}
	if m.ID != 0 {
		return m.ID, nil
	}
	// 被 DoNothing 吞：重读拿已存在记录的 ID。
	var existing models.OCRRulePackage
	if err := r.db.WithContext(ctx).
		Where("name = ?", pkg.Name).
		First(&existing).Error; err != nil {
		return 0, fmt.Errorf("post-conflict lookup existing rule package %q: %w", pkg.Name, err)
	}
	return existing.ID, nil
}

func (r *OCRRulePackageRepo) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&models.OCRRulePackage{}, id).Error
}

func (r *OCRRulePackageRepo) ListVersions(ctx context.Context, id uint) ([]ocrasset.RulePackageVersion, error) {
	var items []models.OCRRulePackageVersion
	if err := r.db.WithContext(ctx).Where("package_id = ?", id).Order("version DESC").Find(&items).Error; err != nil {
		return nil, err
	}
	out := make([]ocrasset.RulePackageVersion, len(items))
	for i, m := range items {
		out[i] = rulePackageVersionToDomain(m)
	}
	return out, nil
}

func (r *OCRRulePackageRepo) RestoreVersion(ctx context.Context, id uint, version int) (*ocrasset.RulePackage, error) {
	var restored models.OCRRulePackage
	err := r.db.WithContext(ctx).Transaction(func(tx *gormLib.DB) error {
		var current models.OCRRulePackage
		if err := tx.First(&current, id).Error; err != nil {
			return err
		}
		var item models.OCRRulePackageVersion
		if err := tx.Where("package_id = ? AND version = ?", id, version).First(&item).Error; err != nil {
			return err
		}
		if err := createRulePackageVersion(ctx, tx, &current); err != nil {
			return err
		}
		if err := json.Unmarshal(item.Snapshot, &restored); err != nil {
			return fmt.Errorf("decode rule package snapshot: %w", err)
		}
		restored.ID = id
		restored.CreatedAt = current.CreatedAt
		restored.UpdatedAt = current.UpdatedAt
		return tx.Save(&restored).Error
	})
	if err != nil {
		return nil, err
	}
	d := rulePackageToDomain(restored)
	return &d, nil
}

func createRulePackageVersion(ctx context.Context, tx *gormLib.DB, pkg *models.OCRRulePackage) error {
	var maxVersion int
	if err := tx.WithContext(ctx).Model(&models.OCRRulePackageVersion{}).
		Where("package_id = ?", pkg.ID).
		Select("COALESCE(MAX(version), 0)").
		Scan(&maxVersion).Error; err != nil {
		return err
	}
	data, err := json.Marshal(pkg)
	if err != nil {
		return err
	}
	return tx.WithContext(ctx).Create(&models.OCRRulePackageVersion{
		PackageID: pkg.ID,
		Version:   maxVersion + 1,
		Snapshot:  datatypes.JSON(data),
	}).Error
}
