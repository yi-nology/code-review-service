package modules

import (
	"context"
	"fmt"

	"github.com/yi-nology/code-review-service/internal/config"
	"github.com/yi-nology/code-review-service/internal/repository/postgres"

	"go.uber.org/fx"
	"gorm.io/gorm"
)

var DatabaseModule = fx.Options(
	fx.Provide(NewDatabase),
)

func NewDatabase(lc fx.Lifecycle, cfg *config.Config) (*gorm.DB, error) {
	db, err := postgres.InitDB(cfg.GetDatabaseConfig())
	if err != nil {
		return nil, fmt.Errorf("init db failed: %w", err)
	}
	lc.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			return postgres.CloseDB(db)
		},
	})
	return db, nil
}
