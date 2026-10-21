package postgres

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/yi-nology/code-review-service/internal/domain/ocrsettings"
	"github.com/yi-nology/code-review-service/internal/repository/postgres/models"

	"github.com/oekit/pkg/cache"

	gormLib "gorm.io/gorm"
)

// ocrSettingsCacheTTL 是 Get 内存缓存的过期时间。
// 取 30s：覆盖 LLM 30s 周期探活（probe 每次调 2 次 Get），让 probe 完全走 cache。
// Save 会立即 invalidate，所以用户在 Web UI 改 settings 后立刻看到效果。
const ocrSettingsCacheTTL = 30 * time.Second

// ocrSettingsCacheKey 是 OCR settings 缓存的固定 key（全局只有这一份设置）。
const ocrSettingsCacheKey = "ocr_settings"

// ocrSettingsCacheNow 是缓存 TTL 判定用的时钟（缺省 time.Now）。
//
// 测试注入假时钟即可**确定性**地驱动过期窗口，无需 sleep（oekit/pkg/cache
// 的 LocalConfig.Now 同款口径：假时钟驱动过期/空值窗口，无 sleep 无 flake）。
var ocrSettingsCacheNow func() time.Time

type OCRSettingsDefaults struct {
	Enabled         bool
	Binary          string
	ContextWindow   int
	MaxOutputTokens int
}

type OCRSettingsRepo struct {
	db       *gormLib.DB
	defaults OCRSettingsDefaults

	// settingsCache 是 OCR settings 的本地缓存（Cache-Aside）。
	//
	// 此前是手搓的「RWMutex + cached + cachedAt + singleflight」四件套：
	// TTL 判定、拷贝语义、并发回源合并各写一份，边界（命中是否续期、
	// 错误是否入缓存、并发 miss 是否只查一次库）全靠注释维持。
	//
	// 现在用 oekit/pkg/cache：LocalCache 提供 TTL/读命中拷贝（CloneOnRead），
	// 外层 SingleFlightCache 装饰器提供并发回源合并（同一 key 只回源一次）。
	// 语义与旧实现逐条对齐：
	//   - TTL 从**写入**时起算，读命中不续期（探活每 30s 才回源一次）；
	//   - 回源失败不写缓存，错误原样上抛；
	//   - 无记录时回源返回默认值（与旧实现一致，同样入缓存）。
	//
	// cacheInner 单独持有是为了拿到 Close（Cache 接口面上没有这个方法）。
	cache      *cache.SingleFlightCache[ocrsettings.Settings]
	cacheInner *cache.LocalCache[ocrsettings.Settings]
}

func NewOCRSettingsRepo(db *gormLib.DB, defaults ...OCRSettingsDefaults) *OCRSettingsRepo {
	repo := &OCRSettingsRepo{db: db}
	if len(defaults) > 0 {
		repo.defaults = defaults[0]
	}
	now := ocrSettingsCacheNow
	if now == nil {
		now = time.Now
	}
	repo.cacheInner = cache.NewLocalCache(cache.LocalConfig[ocrsettings.Settings]{
		TTL: ocrSettingsCacheTTL,
		Now: now,
		// 单键缓存：过期条目由读路径惰性删除即可，不必为它常驻一个
		// 后台清扫协程（也就不必给仓储容器加 Close 生命周期钩子）。
		NoBackgroundSweep: true,
		// 读命中返回副本：调用方改写返回值不会污染缓存里那份
		// （旧实现是手工拷贝一份再返回）。
		CloneOnRead: func(s *ocrsettings.Settings) *ocrsettings.Settings {
			if s == nil {
				return nil
			}
			cp := *s
			return &cp
		},
		Source: repo.loadSettings,
	})
	repo.cache = cache.NewSingleFlightCache(repo.cacheInner)
	return repo
}

// Get 读取 OCR settings（带 30s 本地缓存）。
func (r *OCRSettingsRepo) Get(ctx context.Context) (*ocrsettings.Settings, error) {
	return r.cache.GetOrLoad(ctx, ocrSettingsCacheKey)
}

// loadSettings 是缓存的回源函数：读 DB 最新一条；无记录则回默认值。
func (r *OCRSettingsRepo) loadSettings(ctx context.Context, _ string) (*ocrsettings.Settings, error) {
	var settings models.OCRSettings
	err := r.db.WithContext(ctx).
		Order("id DESC").
		First(&settings).Error
	if err != nil {
		if errors.Is(err, gormLib.ErrRecordNotFound) {
			return r.defaultSettings(), nil
		}
		return nil, err
	}
	d := ocrSettingsToDomain(settings)
	return &d, nil
}

// Close 释放缓存资源（幂等）。
//
// 当前实现用 NoBackgroundSweep（不起后台清扫协程），故无协程可停；
// 保留该方法是为了与 LocalCache 的资源契约对齐，也方便将来打开后台清扫
// 时无需改调用方。
func (r *OCRSettingsRepo) Close(ctx context.Context) error {
	if r.cacheInner == nil {
		return nil
	}
	return r.cacheInner.Close(ctx)
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
		Language:        "中文",
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
		// 清缓存：让 web UI 的改动立即生效（不等 30s TTL 自然过期）。
		if delErr := r.cache.Del(ctx, ocrSettingsCacheKey); delErr != nil {
			return delErr
		}
	}
	return err
}
