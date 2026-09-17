package modules

import (
	"context"
	"time"

	"github.com/yi-nology/code-review-service/internal/argusclient"
	"github.com/yi-nology/code-review-service/internal/config"
	"github.com/yi-nology/code-review-service/internal/repository"
	svcReview "github.com/yi-nology/code-review-service/internal/service/review"

	"github.com/oekit/concurrency/async"
	"github.com/oekit/logx"
	"go.uber.org/fx"
)

var WorkerModule = fx.Options(
	fx.Invoke(StartWorkers),
	fx.Invoke(StartArgusPoller),
)

// StartArgusPoller argus 任务失败兜底轮询（argus 失败终态不回推 sink，
// 靠本轮询把 engine=argus 的 running 记录在 argus 任务失败后收敛到 failed）。
func StartArgusPoller(lc fx.Lifecycle, repos *repository.RepoContainer, argus *argusclient.Client) {
	if !argus.Enabled() {
		return
	}
	poller := svcReview.NewArgusPoller(repos.Review, argus, 30*time.Second)
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			async.GoSafe(func() { poller.Run(ctx) })
			return nil
		},
	})
}

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
