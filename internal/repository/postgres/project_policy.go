package postgres

import (
	"context"
	"errors"

	"github.com/yi-nology/code-review-service/internal/domain/projectpolicy"
	"github.com/yi-nology/code-review-service/internal/repository/postgres/models"
	"github.com/yi-nology/code-review-service/internal/repository/repoerr"
	"github.com/oekit/logx"

	gormLib "gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type ProjectPolicyRepo struct {
	db *gormLib.DB
}

func NewProjectPolicyRepo(db *gormLib.DB) *ProjectPolicyRepo {
	return &ProjectPolicyRepo{db: db}
}

func (r *ProjectPolicyRepo) List(ctx context.Context) ([]projectpolicy.ProjectPolicy, error) {
	var policies []models.ProjectPolicy
	if err := r.db.WithContext(ctx).
		Order("provider, repo").
		Find(&policies).Error; err != nil {
		return nil, err
	}
	out := make([]projectpolicy.ProjectPolicy, len(policies))
	for i, m := range policies {
		out[i] = projectPolicyToDomain(m)
	}
	return out, nil
}

func (r *ProjectPolicyRepo) GetByProviderRepo(ctx context.Context, provider, repo string) (*projectpolicy.ProjectPolicy, error) {
	var m models.ProjectPolicy
	err := r.db.WithContext(ctx).
		Where("provider = ? AND repo = ?", provider, repo).
		First(&m).Error
	if err != nil {
		return nil, wrapNotFound(err)
	}
	d := projectPolicyToDomain(m)
	return &d, nil
}

// GetOrCreateDefault 读取项目策略；不存在时自动创建默认记录（FailOn=major）。
// 并发安全：用 ON CONFLICT DO NOTHING 处理「多个 webhook 同时为同一新仓库建策略」的情况，
// 冲突时不报错，最终以重读结果为准。创建失败（非冲突）只 warn，返回内存默认值，不阻断审查。
func (r *ProjectPolicyRepo) GetOrCreateDefault(ctx context.Context, provider, repo string) (projectpolicy.ProjectPolicy, error) {
	// 1. 命中已有记录直接返回。
	if existing, err := r.GetByProviderRepo(ctx, provider, repo); err == nil && existing != nil {
		return *existing, nil
	} else if err != nil && !errors.Is(err, repoerr.ErrNotFound) {
		// 非 NotFound 的真实错误：返回内存默认值，不尝试创建（DB 可能不可用）。
		return projectpolicy.NewDefault(provider, repo), err
	}

	// 2. NotFound：尝试插入默认记录。ON CONFLICT DO NOTHING 保证并发下不报错。
	def := projectpolicy.NewDefault(provider, repo)
	m := projectPolicyToModel(&def)
	if err := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "provider"}, {Name: "repo"}}, DoNothing: true}).
		Create(&m).Error; err != nil {
		logx.Warn("failed to auto-create default project policy", "provider", provider, "repo", repo, "error", err)
		// 不阻断审查：返回内存默认值。重读可能拿到并发对手建的记录，拿不到也无妨。
	}

	// 3. 重读：处理「我 Create 被 DoNothing 吞、但并发对手已成功建记录」的情况。
	if created, err := r.GetByProviderRepo(ctx, provider, repo); err == nil && created != nil {
		return *created, nil
	}
	return def, nil
}

func (r *ProjectPolicyRepo) Save(ctx context.Context, policy *projectpolicy.ProjectPolicy) error {
	m := projectPolicyToModel(policy)
	return r.db.WithContext(ctx).Transaction(func(tx *gormLib.DB) error {
		var existing models.ProjectPolicy
		err := tx.Unscoped().
			Where("provider = ? AND repo = ?", m.Provider, m.Repo).
			First(&existing).Error
		if errors.Is(err, gormLib.ErrRecordNotFound) {
			return tx.Create(&m).Error
		}
		if err != nil {
			return err
		}
		m.ID = existing.ID
		return tx.Unscoped().Save(&m).Error
	})
}

func (r *ProjectPolicyRepo) Delete(ctx context.Context, provider, repo string) error {
	// Unscoped 硬删除：项目策略无历史版本需求（不像 rule packages 需要版本快照），
	// 软删除会让 List（默认过滤 deleted_at IS NULL）看不到已删策略，前端显示"暂无"，
	// 但 GetOrCreateDefault（ON CONFLICT DO NOTHING 不查软删除）仍会创建新策略，造成数据割裂。
	// 硬删除 + (provider, repo) uniqueIndex 保证幂等。
	result := r.db.WithContext(ctx).Unscoped().
		Where("provider = ? AND repo = ?", provider, repo).
		Delete(&models.ProjectPolicy{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gormLib.ErrRecordNotFound
	}
	return nil
}
