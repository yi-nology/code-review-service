package postgres

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/yi-nology/code-review-service/internal/repository"
	"github.com/yi-nology/code-review-service/internal/repository/postgres/models"

	"github.com/oekit/logx"
	"gorm.io/gorm"
)

// LeaseRepo 租约表实现：ON CONFLICT 条件更新的原子抢租。
type LeaseRepo struct {
	db     *gorm.DB
	holder string
}

// 接口契约锚点。
var (
	_ repository.LeaseRepository           = (*LeaseRepo)(nil)
	_ repository.MemoryNamespaceRepository = (*MemoryNSRepo)(nil)
)

// NewLeaseRepo 构造租约仓储；holder 标识持有者便于诊断（hostname+pid）。
func NewLeaseRepo(db *gorm.DB) *LeaseRepo {
	h, err := os.Hostname()
	if err != nil {
		h = "unknown"
	}
	return &LeaseRepo{db: db, holder: fmt.Sprintf("%s:%d", h, os.Getpid())}
}

// TryAcquire 原子抢租：冲突时仅当现有租约已过期才覆盖。
func (r *LeaseRepo) TryAcquire(ctx context.Context, name string, ttl time.Duration) bool {
	expires := time.Now().UTC().Add(ttl)
	res := r.db.WithContext(ctx).Exec(`
INSERT INTO job_leases (name, holder, expires_at, updated_at)
VALUES (?, ?, ?, now())
ON CONFLICT (name) DO UPDATE
SET holder = EXCLUDED.holder, expires_at = EXCLUDED.expires_at, updated_at = now()
WHERE job_leases.expires_at < now()`, name, r.holder, expires)
	if res.Error != nil {
		// 抢租失败按「没抢到」处理：任务下一 tick 再试，不阻断主循环。
		logx.CtxWarnf(ctx, "lease acquire query failed (treat as not-acquired): %v", res.Error)
		return false
	}
	return res.RowsAffected == 1
}

// MemoryNSRepo 记忆库命名空间注册表实现。
type MemoryNSRepo struct {
	db *gorm.DB
}

func NewMemoryNSRepo(db *gorm.DB) *MemoryNSRepo { return &MemoryNSRepo{db: db} }

func (r *MemoryNSRepo) TouchNS(ctx context.Context, ns string) error {
	return r.db.WithContext(ctx).Exec(`
INSERT INTO memory_namespaces (namespace, last_seen_at, created_at)
VALUES (?, now(), now())
ON CONFLICT (namespace) DO UPDATE SET last_seen_at = now()`, ns).Error
}

func (r *MemoryNSRepo) ListNS(ctx context.Context) ([]string, error) {
	rows := []models.MemoryNamespace{}
	err := r.db.WithContext(ctx).Order("last_seen_at DESC").Limit(1000).Find(&rows).Error
	out := make([]string, 0, len(rows))
	for _, row := range rows {
		out = append(out, row.Namespace)
	}
	return out, err
}
