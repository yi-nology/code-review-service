package postgres

import (
	"fmt"
	"time"

	"github.com/yi-nology/code-review-service/internal/config"
	"github.com/yi-nology/code-review-service/internal/repository/postgres/models"
	"github.com/oekit/gormx"

	"github.com/cloudwego/hertz/pkg/common/hlog"
	gormLib "gorm.io/driver/postgres"
	gormDB "gorm.io/gorm"
)

func InitDB(cfg *config.DatabaseConfig) (*gormDB.DB, error) {
	db, err := gormx.OpenWithPool(
		gormLib.Open(cfg.DSN),
		cfg.MaxIdleConns,
		cfg.MaxOpenConns,
		30*time.Minute,
	)
	if err != nil {
		return nil, fmt.Errorf("init database: %w", err)
	}

	if cfg.AutoMigrate == nil || *cfg.AutoMigrate {
		if err := db.AutoMigrate(
			&models.Review{},
			&models.OCRSettings{},
			&models.ProjectPolicy{},
			&models.GitPlatform{},
			&models.WebhookDelivery{},
			&models.ChangeRevision{},
			&models.WritebackItem{},
			&models.GlobalRules{},
			&models.ReviewSession{},
			&models.ReviewerAgent{},
			&models.ProjectAgent{},
			&models.ReviewMap{},
			&models.CustomProvider{},
		); err != nil {
			return nil, fmt.Errorf("auto migrate: %w", err)
		}
		hlog.Infof("database auto-migration completed")

		// 部分唯一索引：防止并发创建同一 (provider, repo, PR/MR) 的多个 active review（#24 TOCTOU 根因修复）。
		// GORM AutoMigrate 不支持部分索引，需手动 SQL。IF NOT EXISTS 幂等。
		// 失败时（通常因历史已有重复 active 数据）仅告警不阻断启动，需人工清理后再重启。
		if err := db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_reviews_active_change
			ON reviews (provider, repo, (COALESCE(request->>'pull_request_id', request->>'merge_request_iid')))
			WHERE status IN ('queued', 'running')
			AND COALESCE(request->>'pull_request_id', request->>'merge_request_iid') != ''`).Error; err != nil {
			hlog.Warnf("failed to create active review partial index (可能存在历史重复 active review，需人工清理): %v", err)
		}
	} else {
		hlog.Infof("database auto-migration disabled")
	}

	hlog.Infof("database initialized successfully")
	return db, nil
}

func CloseDB(db *gormDB.DB) error {
	if db == nil {
		return nil
	}
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}
