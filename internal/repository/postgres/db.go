package postgres

import (
	"fmt"
	"time"

	"github.com/oekit/gormx"
	"github.com/yi-nology/code-review-service/internal/config"
	"github.com/yi-nology/code-review-service/internal/repository/postgres/models"

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

	if err := db.AutoMigrate(
		&models.Review{},
		&models.OCRSettings{},
		&models.ProjectPolicy{},
		&models.GitPlatform{},
		&models.WebhookDelivery{},
		&models.ChangeRevision{},
		&models.WritebackItem{},
		&models.GlobalRules{},
	); err != nil {
		return nil, fmt.Errorf("auto migrate: %w", err)
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
