package modules

import (
	"context"
	"time"

	"github.com/yi-nology/code-review-service/internal/config"
	"github.com/yi-nology/code-review-service/internal/service/cleanup"

	"go.uber.org/fx"
)

var CleanupModule = fx.Options(
	fx.Invoke(registerWorkspaceCleaner),
)

func registerWorkspaceCleaner(cfg *config.Config, lc fx.Lifecycle) {
	if !cfg.Review.WorkspaceCleanup.Enabled {
		return
	}

	maxAge := 24 * time.Hour
	if cfg.Review.WorkspaceCleanup.MaxAge != "" {
		if d, err := time.ParseDuration(cfg.Review.WorkspaceCleanup.MaxAge); err == nil {
			maxAge = d
		}
	}

	var intervalOpt cleanup.Option
	if cfg.Review.WorkspaceCleanup.Interval != "" {
		if d, err := time.ParseDuration(cfg.Review.WorkspaceCleanup.Interval); err == nil {
			intervalOpt = cleanup.WithInterval(d)
		}
	}

	cleaner := cleanup.NewWorkspaceCleaner(maxAge, intervalOpt)

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			cleaner.Start()
			return nil
		},
		OnStop: func(ctx context.Context) error {
			cleaner.Stop()
			return nil
		},
	})
}
