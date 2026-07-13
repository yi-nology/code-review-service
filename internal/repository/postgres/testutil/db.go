// Package testutil 提供 postgres 仓储层集成测试的 DB 安全网（Phase 0）。
//
// 用 testcontainers 起一次性 postgres 容器 + AutoMigrate + 建好生产同款的部分唯一索引，
// 让 internal/repository/postgres 下的 19 个实现可被真实 DB 回归覆盖。
// docker 不可用时由 main_test.go 的 TestMain 整包跳过。
package testutil

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/yi-nology/code-review-service/internal/repository/postgres/models"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	tcwait "github.com/testcontainers/testcontainers-go/wait"
	gormLib "gorm.io/driver/postgres"
	gormDB "gorm.io/gorm"
)

// AllModels 是 AutoMigrate 的完整模型列表，与 db.go InitDB 保持一致。
var AllModels = []any{
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
}

// activeReviewIndexSQL 与 db.go 的部分唯一索引完全一致（#24 TOCTOU 修复）。
const activeReviewIndexSQL = `CREATE UNIQUE INDEX IF NOT EXISTS idx_reviews_active_change
	ON reviews (provider, repo,
		(COALESCE(NULLIF(pull_request_id, ''), NULLIF(merge_request_iid, ''), request->>'pull_request_id', request->>'merge_request_iid')))
	WHERE status IN ('queued', 'running')
	AND COALESCE(NULLIF(pull_request_id, ''), NULLIF(merge_request_iid, ''), request->>'pull_request_id', request->>'merge_request_iid') != ''`

// StartPostgres 启动一次性 postgres 容器、跑 AutoMigrate 并建好部分唯一索引。
// 调用方（通常是 TestMain）负责在 m.Run() 后 defer Terminate。
func StartPostgres(ctx context.Context) (testcontainers.Container, *gormDB.DB, error) {
	pgContainer, err := postgres.Run(ctx,
		"postgres:16-alpine",
		postgres.WithDatabase("review_test"),
		postgres.WithUsername("postgres"),
		postgres.WithPassword("postgres"),
		testcontainers.WithWaitStrategy(
			tcwait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(40*time.Second),
		),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("start postgres container: %w", err)
	}

	connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		_ = pgContainer.Terminate(ctx)
		return nil, nil, fmt.Errorf("get connection string: %w", err)
	}

	db, err := gormDB.Open(gormLib.Open(connStr), &gormDB.Config{})
	if err != nil {
		_ = pgContainer.Terminate(ctx)
		return nil, nil, fmt.Errorf("open gorm: %w", err)
	}

	if err := db.AutoMigrate(AllModels...); err != nil {
		_ = pgContainer.Terminate(ctx)
		return nil, nil, fmt.Errorf("auto migrate: %w", err)
	}

	if err := db.Exec(activeReviewIndexSQL).Error; err != nil {
		_ = pgContainer.Terminate(ctx)
		return nil, nil, fmt.Errorf("create active review index: %w", err)
	}

	return pgContainer, db, nil
}

// Truncate 清空指定表，用于测试间隔离。RESTART IDENTITY 重置自增主键，CASCADE 处理外键。
func Truncate(db *gormDB.DB, tables ...string) error {
	if len(tables) == 0 {
		return nil
	}
	quoted := make([]string, len(tables))
	for i, t := range tables {
		quoted[i] = `"` + t + `"`
	}
	return db.Exec("TRUNCATE TABLE " + strings.Join(quoted, ", ") + " RESTART IDENTITY CASCADE").Error
}
