package postgres

import (
	"fmt"
	"time"

	"github.com/yi-nology/code-review-service/internal/config"
	"github.com/yi-nology/code-review-service/internal/repository/postgres/models"
	"github.com/oekit/gormx"

	"github.com/oekit/logx"
	gormLib "gorm.io/driver/postgres"
	gormDB "gorm.io/gorm"
)

func InitDB(cfg *config.DatabaseConfig) (*gormDB.DB, error) {
	db, err := gormx.Open(
		gormLib.Open(cfg.DSN),
		gormx.WithConnectionPool(cfg.MaxIdleConns, cfg.MaxOpenConns, 30*time.Minute),
	)
	if err != nil {
		return nil, fmt.Errorf("init database: %w", err)
	}
	if sqlDB, err := db.DB(); err == nil {
		sqlDB.SetConnMaxIdleTime(5 * time.Minute) // 空闲连接 5 分钟后回收，防止 DB 端 wait_timeout 静默关闭后取到失效连接
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
			&models.OCRRulePackage{},
			&models.OCRToolPackage{},
			&models.ReviewSession{},
			&models.ReviewerAgent{},
			&models.ProjectAgent{},
			&models.CustomProvider{},
		); err != nil {
			return nil, fmt.Errorf("auto migrate: %w", err)
		}
		logx.Infof("database auto-migration completed")

		// 部分唯一索引：防止并发创建同一 (provider, repo, PR/MR) 的多个 active review（#24 TOCTOU 根因修复）。
		// 优先使用提取列 pull_request_id / merge_request_iid，JSONB 作为回退（兼容历史数据未回填的情况）。
		if err := db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_reviews_active_change
			ON reviews (provider, repo,
				(COALESCE(NULLIF(pull_request_id, ''), NULLIF(merge_request_iid, ''), request->>'pull_request_id', request->>'merge_request_iid')))
			WHERE status IN ('queued', 'running')
			AND COALESCE(NULLIF(pull_request_id, ''), NULLIF(merge_request_iid, ''), request->>'pull_request_id', request->>'merge_request_iid') != ''`).Error; err != nil {
			logx.Warnf("failed to create active review partial index (可能存在历史重复 active review，需人工清理): %v", err)
		}
	} else {
		logx.Infof("database auto-migration disabled")
	}

	logx.Infof("database initialized successfully")
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
