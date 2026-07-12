package postgres

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/yi-nology/code-review-service/internal/domain/ocrsettings"
	"github.com/yi-nology/code-review-service/internal/repository/postgres/models"

	"github.com/oekit/concurrency/singleflight"

	gormLib "gorm.io/gorm"
)

// ocrSettingsCacheTTL 是 Get 内存缓存的过期时间。
// 取 30s：覆盖 LLM 30s 周期探活（probe 每次调 2 次 Get），让 probe 完全走 cache。
// Save 会立即 invalidate，所以用户在 Web UI 改 settings 后立刻看到效果。
const ocrSettingsCacheTTL = 30 * time.Second

type OCRSettingsDefaults struct {
	Enabled         bool
	Binary          string
	ContextWindow   int
	MaxOutputTokens int
}

type OCRSettingsRepo struct {
	db       *gormLib.DB
	defaults OCRSettingsDefaults

	// flight 防止 cache miss 时多个 caller 同时涌入 DB（dogpile / thundering herd）。
	// 同一时刻只允许一个 goroutine 走「DB 读 + 写 cache」路径，其他等待共享结果。
	flight singleflight.Flight[*ocrsettings.Settings]
	mu     sync.RWMutex
	cached *ocrsettings.Settings
	// cachedAt 是最近一次「成功读 DB 并写入 cache」的时间，用于 30s 过期判断。
	// 注意：cache hit 路径不更新此值，确保 cache miss 严格每 30s 一次（不计单进程内多次 hit）。
	cachedAt time.Time
}

func NewOCRSettingsRepo(db *gormLib.DB, defaults ...OCRSettingsDefaults) *OCRSettingsRepo {
	repo := &OCRSettingsRepo{db: db}
	if len(defaults) > 0 {
		repo.defaults = defaults[0]
	}
	return repo
}

func (r *OCRSettingsRepo) Get(ctx context.Context) (*ocrsettings.Settings, error) {
	// 快路径：30s 内 cache hit（只读锁，无竞争开销）。
	r.mu.RLock()
	if r.cached != nil && time.Since(r.cachedAt) < ocrSettingsCacheTTL {
		out := *r.cached
		r.mu.RUnlock()
		return &out, nil
	}
	r.mu.RUnlock()

	// cache miss：所有并发 caller 收敛到 singleflight，同一时刻只 1 个真正查 DB。
	return r.flight.Do(ctx, "ocr_settings", func(ctx context.Context) (*ocrsettings.Settings, error) {
		// 二次检查：进入 flight 后可能上一个 caller 刚写好 cache。
		r.mu.RLock()
		if r.cached != nil && time.Since(r.cachedAt) < ocrSettingsCacheTTL {
			out := *r.cached
			r.mu.RUnlock()
			return &out, nil
		}
		r.mu.RUnlock()

		var settings models.OCRSettings
		err := r.db.WithContext(ctx).
			Order("id DESC").
			First(&settings).Error
		if err != nil {
			if errors.Is(err, gormLib.ErrRecordNotFound) {
				def := r.defaultSettings()
				r.setCache(def)
				return def, nil
			}
			return nil, err
		}
		d := ocrSettingsToDomain(settings)
		r.setCache(&d)
		return &d, nil
	})
}

// setCache 写入缓存（拷贝值，外部修改不会影响 cache）。
func (r *OCRSettingsRepo) setCache(s *ocrsettings.Settings) {
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := *s
	r.cached = &cp
	r.cachedAt = time.Now()
}

// invalidateCache 清缓存。Save 后调用，让 web UI 改动立即生效。
func (r *OCRSettingsRepo) invalidateCache() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.cached = nil
	r.cachedAt = time.Time{}
}

func (r *OCRSettingsRepo) defaultSettings() *ocrsettings.Settings {
	binary := strings.TrimSpace(r.defaults.Binary)
	if binary == "" {
		binary = "ocr"
	}
	return &ocrsettings.Settings{
		Enabled:         r.defaults.Enabled,
		Binary:          binary,
		Audience:        "agent",
		DefaultSeverity: "major",
		Language:        "Chinese",
		Concurrency:     2,
		MaxGitProcs:     4,
		TimeoutMinutes:  10,
		ReasoningMode:   "light",
		ContextWindow:   int64(r.defaults.ContextWindow),
		MaxOutputTokens: int64(r.defaults.MaxOutputTokens),
	}
}

func (r *OCRSettingsRepo) Save(ctx context.Context, settings *ocrsettings.Settings) error {
	m := ocrSettingsToModel(settings)
	err := r.db.WithContext(ctx).Transaction(func(tx *gormLib.DB) error {
		var existing models.OCRSettings
		err := tx.Order("id DESC").First(&existing).Error
		if errors.Is(err, gormLib.ErrRecordNotFound) {
			return tx.Create(&m).Error
		}
		if err != nil {
			return err
		}
		m.ID = existing.ID
		return tx.Save(&m).Error
	})
	if err == nil {
		r.invalidateCache()
	}
	return err
}
