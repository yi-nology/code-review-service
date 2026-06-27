package modules

import (
	"context"

	"github.com/yi-nology/code-review-service/internal/config"
	svcReview "github.com/yi-nology/code-review-service/internal/service/review"

	"github.com/oekit/concurrency/async"
	"github.com/oekit/logx"
	"go.uber.org/fx"
)

var WorkerModule = fx.Options(
	fx.Invoke(StartWorkers),
)

func StartWorkers(lc fx.Lifecycle, cfg *config.Config, reviewSvc *svcReview.Service) {
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			async.GoSafe(func() {
				reviewSvc.RunWorkers(
					ctx,
					cfg.Review.WorkerConcurrency,
					cfg.GetWorkerPollInterval(),
					cfg.GetWorkerStaleAfter(),
				)
			})
			logx.Infof("background workers started with concurrency=%d", cfg.Review.WorkerConcurrency)
			return nil
		},
		OnStop: func(ctx context.Context) error {
			logx.Info("stopping review workers, draining in-flight reviews...")
			if err := reviewSvc.Shutdown(ctx); err != nil {
				logx.Warn("review workers shutdown timed out, some in-flight reviews may be incomplete", "error", err)
				return nil
			}
			logx.Info("review workers stopped cleanly")
			return nil
		},
	})
}
