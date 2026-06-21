package modules

import (
	"context"

	"github.com/yi-nology/code-review-service/internal/config"
	svcReview "github.com/yi-nology/code-review-service/internal/service/review"

	"github.com/oekit/concurrency/async"
	"github.com/cloudwego/hertz/pkg/common/hlog"
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
			hlog.Infof("background workers started with concurrency=%d", cfg.Review.WorkerConcurrency)
			return nil
		},
	})
}
