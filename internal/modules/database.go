package modules

import (
	"context"

	"github.com/yi-nology/code-review-service/internal/config"
	"github.com/yi-nology/code-review-service/internal/repository/postgres"

	"github.com/cloudwego/hertz/pkg/common/hlog"
	"go.uber.org/fx"
	"gorm.io/gorm"
)

var DatabaseModule = fx.Options(
	fx.Provide(NewDatabase),
)

func NewDatabase(lc fx.Lifecycle, cfg *config.Config) (*gorm.DB, error) {
	db, err := postgres.InitDB(cfg.GetDatabaseConfig())
	if err != nil {
		hlog.Fatalf("init db failed: %v", err)
	}
	lc.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			return postgres.CloseDB(db)
		},
	})
	return db, nil
}
