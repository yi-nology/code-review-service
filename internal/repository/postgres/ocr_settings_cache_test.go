package postgres

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/yi-nology/code-review-service/internal/domain/ocrsettings"

	gormDB "gorm.io/gorm"
)

// queryCountingDB 在底层 *gormDB.DB 上注册一个 query callback，对每一次 SQL query
// 自增计数。专供 OCR settings TTL cache 行为测试：证明 30s 内多次 Get 只查 1 次 DB。
//
// callback 用闭包变量（而非 Register 名字）自增计数，避免 gorm 同名 callback 重复注册告警。
type queryCountingDB struct {
	*gormDB.DB
	counter *int64
}

func newQueryCountingDB(db *gormDB.DB) *queryCountingDB {
	var c int64
	q := &queryCountingDB{DB: db, counter: &c}
	if db != nil {
		_ = db.Callback().Query().Before("*").Register("ocr_settings_test_count", func(tx *gormDB.DB) {
			atomic.AddInt64(q.counter, 1)
		})
	}
	return q
}

func (q *queryCountingDB) queryCount() int64 {
	return atomic.LoadInt64(q.counter)
}

// TestOCRSettingsRepo_GetCachesAcrossCallsWithinTTL 验证 30s 内多次 Get 只查 1 次 DB。
// 这是 fc9ba34 的核心修复点：之前每 30s 探活查 2 次（24h 5760 次），现在 24h ~96 次。
func TestOCRSettingsRepo_GetCachesAcrossCallsWithinTTL(t *testing.T) {
	truncateTables(t, "ocr_settings")
	ctx := context.Background()
	q := newQueryCountingDB(testDB)
	repo := NewOCRSettingsRepo(q.DB)

	// 第一次 Get：cache miss → 查 DB → 写 cache
	got, err := repo.Get(ctx)
	if err != nil {
		t.Fatalf("first Get: %v", err)
	}
	if got == nil {
		t.Fatal("first Get returned nil")
	}
	if c := q.queryCount(); c != 1 {
		t.Fatalf("first Get query count = %d, want 1 (cache miss + write cache path)", c)
	}

	// 连续 9 次 cache hit 路径（不调 DB）
	for i := 0; i < 9; i++ {
		_, err := repo.Get(ctx)
		if err != nil {
			t.Fatalf("cached Get #%d: %v", i+2, err)
		}
	}
	if c := q.queryCount(); c != 1 {
		t.Fatalf("after 9 cached Gets, total query count = %d, want 1 (cache must prevent DB hits)", c)
	}
}

// TestOCRSettingsRepo_SaveInvalidatesCache 验证 Save 之后立即 Get 重新查 DB，
// 让 Web UI 改 settings 立刻生效（不是 30s 后才看到）。
//
// Save 自身会查一次 DB（Order("id DESC").First 找 existing record），所以 4 次 query 是预期：
//   - Get#1 (miss): 1
//   - Get#2 (hit):  0
//   - Save 内部:    1
//   - Get#3 (miss 因 invalidate): 1
func TestOCRSettingsRepo_SaveInvalidatesCache(t *testing.T) {
	truncateTables(t, "ocr_settings")
	ctx := context.Background()
	q := newQueryCountingDB(testDB)
	repo := NewOCRSettingsRepo(q.DB)

	if _, err := repo.Get(ctx); err != nil {
		t.Fatalf("first Get: %v", err)
	}
	if c := q.queryCount(); c != 1 {
		t.Fatalf("first Get query count = %d, want 1", c)
	}
	// 第二次 Get 应 cache hit
	if _, err := repo.Get(ctx); err != nil {
		t.Fatalf("cached Get: %v", err)
	}
	if c := q.queryCount(); c != 1 {
		t.Fatalf("cached Get should not query DB, got %d", c)
	}

	// Save 一个新 setting → 必须立即 invalidate cache
	newSettings := &ocrsettings.Settings{
		Enabled:         true,
		Binary:          "ocr",
		Audience:        "agent",
		DefaultSeverity: "minor",
		Language:        "English",
		Concurrency:     3,
		MaxGitProcs:     5,
		TimeoutMinutes:  20,
		ReasoningMode:   "deep",
		ContextWindow:   200000,
		MaxOutputTokens: 16384,
	}
	if err := repo.Save(ctx, newSettings); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// Save 之后立即 Get → 必须重新查 DB（cache invalidate）
	got, err := repo.Get(ctx)
	if err != nil {
		t.Fatalf("Get after Save: %v", err)
	}
	if !got.Enabled {
		t.Fatal("Get after Save should return Enabled=true from DB")
	}
	// 期望 3: Get#1 (1) + Save 内部 find existing (1) + Get#3 invalidate miss (1)
	if c := q.queryCount(); c != 3 {
		t.Fatalf("total query count = %d, want 3 (1 Get + 1 Save-find + 1 Get-after-invalidate)", c)
	}
}

// TestOCRSettingsRepo_CacheExpiresAfterTTL 验证 30s 后 cache 过期。
// 用极小 TTL 模拟（不真等 30s），通过覆盖 package-level 常量做不到，
// 所以这里改测「cache hit 在新写入后立即被识别为命中」+「手工 setCache + 改时间」 验证过期路径。
// 直接覆盖 r.cachedAt 到 30s 之前，验证下次 Get 走 miss 路径。
func TestOCRSettingsRepo_CacheExpiresAfterTTL(t *testing.T) {
	truncateTables(t, "ocr_settings")
	ctx := context.Background()
	q := newQueryCountingDB(testDB)
	repo := NewOCRSettingsRepo(q.DB)

	// 第一次 Get 写 cache
	if _, err := repo.Get(ctx); err != nil {
		t.Fatalf("first Get: %v", err)
	}
	if c := q.queryCount(); c != 1 {
		t.Fatalf("first Get query count = %d, want 1", c)
	}

	// 手动把 cachedAt 改到 31s 之前，模拟 30s 过期
	repo.mu.Lock()
	repo.cachedAt = time.Now().Add(-31 * time.Second)
	repo.mu.Unlock()

	// 下次 Get 必须走 miss 路径 → 再查 DB
	if _, err := repo.Get(ctx); err != nil {
		t.Fatalf("Get after expiry: %v", err)
	}
	if c := q.queryCount(); c != 2 {
		t.Fatalf("Get after expiry query count = %d, want 2 (expired cache must miss)", c)
	}
}
