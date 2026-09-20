package modules

import (
	"context"
	"time"

	"github.com/yi-nology/code-review-service/internal/argusclient"
	"github.com/yi-nology/code-review-service/internal/config"
	"github.com/yi-nology/code-review-service/internal/repository"
	gitProvider "github.com/yi-nology/code-review-service/internal/service/provider"
	svcReview "github.com/yi-nology/code-review-service/internal/service/review"
	reviewdomain "github.com/yi-nology/code-review-service/internal/service/review/domain"
	"github.com/yi-nology/code-review-service/internal/service/statuscheck"

	"github.com/oekit/concurrency/async"
	"github.com/oekit/logx"
	"go.uber.org/fx"
)

var WorkerModule = fx.Options(
	fx.Invoke(StartWorkers),
	fx.Invoke(StartArgusPoller),
)

// StartArgusPoller argus 任务失败兜底轮询（argus 失败终态不回推 sink，
// 靠本轮询把 engine=argus 的 running 记录在 argus 任务失败后收敛到 failed；
// 回推丢失时拉全量任务详情恢复落库），并经 notifier 发应用内通知、
// statusPoster 在 PR 上留门禁 status。
func StartArgusPoller(lc fx.Lifecycle, repos *repository.RepoContainer, argus *argusclient.Client,
	notifier reviewdomain.Notifier, gitSvc *gitProvider.GitProviderService, cfg *config.Config) {
	if !argus.Enabled() {
		return
	}
	poller := svcReview.NewArgusPoller(repos.Review, repos.ArgusReview, argus, notifier,
		statuscheck.NewPoster(gitSvc, cfg.ExternalBaseURL), 30*time.Second)
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
