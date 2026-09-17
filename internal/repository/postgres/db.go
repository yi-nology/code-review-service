package postgres

import (
	"fmt"
	"time"

	"github.com/yi-nology/code-review-service/internal/config"
	"github.com/yi-nology/code-review-service/internal/repository/postgres/models"
	"github.com/oekit/gormx" //lint:ignore SA1019 PageResponse 未废弃，上游包级别注释误标

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
			&models.OCRRulePackageVersion{},
			&models.OCRToolPackageVersion{},
			&models.ReviewSession{},
			&models.ReviewerAgent{},
			&models.ProjectAgent{},
			&models.CustomProvider{},
			&models.FindingFeedback{},
			&models.Notification{},
			&models.APIToken{},
			&models.AuditLog{},
			&models.LLMModelPricing{},
			&models.ArgusReview{},
			&models.ArgusFinding{},
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

		// Argus 承接：(review, fp) 唯一只约束非空 fp——空 fp 的旧行允许多条共存，
		// 否则一次上报多条无指纹意见会在第二条撞唯一约束、整组入库失败。
		// 先 DROP 再 CREATE：早期版本的 GORM 模型用 uniqueIndex tag 建过全量唯一
		// 索引（同名），IF NOT EXISTS 会跳过、让坏索引在存量部署上存活——全量
		// 索引不可能存在重复行，先删后建（放宽为部分索引）总是安全的。
		if err := db.Exec(`DROP INDEX IF EXISTS idx_argus_finding_review_fp`).Error; err != nil {
			logx.Warnf("failed to drop legacy argus finding fingerprint index: %v", err)
		}
		if err := db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_argus_finding_review_fp
			ON argus_findings (review_id, fp) WHERE fp <> ''`).Error; err != nil {
			logx.Warnf("failed to create argus finding fingerprint index: %v", err)
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
