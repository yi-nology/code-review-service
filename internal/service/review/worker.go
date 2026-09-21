package review

import (
	"context"
	"fmt"
	"time"

	bizmetrics "github.com/yi-nology/code-review-service/internal/observability/metrics"
	"github.com/yi-nology/code-review-service/internal/service/progress"
	"github.com/yi-nology/code-review-service/internal/service/review/domain"
	"github.com/oekit/concurrency/async"
	"github.com/oekit/concurrency/group"
	"github.com/oekit/logx"
)

func (s *Service) RunWorkers(ctx context.Context, concurrency int, pollInterval, staleAfter time.Duration) {
	if concurrency <= 0 {
		logx.Info("review workers disabled")
		return
	}
	if pollInterval <= 0 {
		pollInterval = time.Second
	}
	if staleAfter <= 0 {
		staleAfter = 15 * time.Minute
	}

	// 启动时：收敛上一实例遗留的孤儿（服务重启后全是孤儿，必须重新处理）。
	s.reapOrphans(ctx, time.Now().UTC().Add(1*time.Hour))

	// 启动回填：修复历史版本「重试超限静默置 failed、不写门禁 status」留下的
	// 卡死 MR（DB 已 failed 但 response.gate 仍 pending 的记录）。幂等：补写后
	// gate 变为 error，下次启动不再命中。
	s.backfillBrokenGateFailures(ctx)

	g := &group.Group{}
	for workerID := 1; workerID <= concurrency; workerID++ {
		id := workerID
		g.Add(
			func() error {
				s.worker(ctx, id, pollInterval)
				return nil
			},
			func(error) {
			},
		)
	}

	async.GoSafe(func() {
		if err := g.Run(); err != nil {
			logx.Error("worker group failed", "error", err)
		}
	})

	// 定期重排卡住的 running 评审（运行时超时保护）
	async.GoSafe(func() {
		ticker := time.NewTicker(staleAfter)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.reapOrphans(ctx, time.Now().UTC().Add(-staleAfter))
			}
		}
	})

	// 定期采集队列深度指标（每 30s 查一次 queued+running 数量）。
	async.GoSafe(func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if depth, err := s.reviewRepo.QueueDepth(ctx); err == nil {
					bizmetrics.ReviewQueueDepth.Set(float64(depth))
				}
			}
		}
	})
}

// reapOrphans 收敛孤儿 running 评审（启动时=上一实例遗留；运行期=长时间无进展）：
//   - 重试达上限的：FailStaleRunning 置 failed 后逐条经 FailureReporter 补写终态
//     （历史/落库/通知/门禁 status）。此前直接 SQL 置 failed 不写 status，
//     GitLab 侧 pending 永不收敛，MR 被永久卡死。
//   - 其余：重新入队等待 worker 再次认领。
func (s *Service) reapOrphans(ctx context.Context, before time.Time) {
	exhausted, err := s.reviewRepo.FailStaleRunning(ctx, before)
	if err != nil {
		logx.Error("failed to reap exhausted stale reviews", "error", err)
	} else if len(exhausted) > 0 {
		logx.Warnf("reaped %d exhausted stale reviews as failed", len(exhausted))
		for _, rec := range exhausted {
			s.failureReporter.Report(ctx, rec.ID, rec.Request, "worker",
				fmt.Errorf("评审被反复中断（服务重启或长时间无进展），已达重试上限，请在控制台重试"), domain.OCRMetadata{})
		}
	}

	n, err := s.reviewRepo.RequeueStale(ctx, before)
	if err != nil {
		logx.Error("failed to requeue orphaned reviews", "error", err)
	} else if n > 0 {
		logx.Warnf("requeued %d orphaned running reviews", n)
	}
}

// backfillBrokenGateFailures 回填历史静默失败的评审：早期版本把重试超限的孤儿
// 直接 SQL 置 failed，不写门禁 status——这些 MR 在 GitLab 上停留在 pending 永久阻塞。
// 以「DB status=failed 且 response.gate 仍 pending」精确圈定受损集合，补走
// FailureReporter；补写后 gate 变 error，天然幂等，不会重复上报。
func (s *Service) backfillBrokenGateFailures(ctx context.Context) {
	const (
		batchSize = 20
		maxTotal  = 200
	)
	patched := 0
	for patched < maxTotal {
		res, err := s.reviewRepo.List(ctx, domain.ListOptions{
			Status:     statusFailed,
			GateStatus: domain.GatePending,
			Page:       1,
			PageSize:   batchSize,
		})
		if err != nil {
			logx.Error("backfill broken gate failures: list failed", "error", err)
			return
		}
		if len(res.Items) == 0 {
			return
		}
		for _, rec := range res.Items {
			if patched >= maxTotal {
				break
			}
			s.failureReporter.Report(ctx, rec.ID, rec.Request, "backfill",
				fmt.Errorf("历史评审失败时未写门禁状态，已补写终态；如需重新审查请在控制台重试"), domain.OCRMetadata{})
			patched++
		}
	}
	if patched > 0 {
		logx.Warnf("backfilled %d broken gate failures with terminal status", patched)
	}
}

// startHeartbeat 启动执行期心跳：每隔 interval 刷新评审 updated_at，返回停止函数。
// 防止长评审（执行期间无落库事件）被 RequeueStale 误判为孤儿——误判会导致同一
// MR 被重复执行，且重试计数耗尽后被淘汰为 failed。
func (s *Service) startHeartbeat(reviewID string, interval time.Duration) (stop func()) {
	if interval <= 0 {
		return func() {}
	}
	done := make(chan struct{})
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				tctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				err := s.reviewRepo.TouchRunning(tctx, reviewID)
				cancel()
				if err != nil {
					logx.Warnf("review heartbeat failed: review_id=%s err=%v", reviewID, err)
				}
			}
		}
	}()
	return func() { close(done) }
}

// Shutdown 通知 worker 停止并等待所有 in-flight review 完成或 ctx 超时。
//
// 由 fx 的 OnStop 钩子调用（见 modules/worker.go），确保停机时：
//  1. worker/stale-requeue goroutine 通过传入的 lifecycle ctx 取消自然退出；
//  2. 阻塞等待所有正在执行的 review 落库完成，避免 DB 连接池被提前关闭导致写失败。
//
// WorkerModule 必须在 DatabaseModule 之后注册（见 init.go），以保证本方法
// 的 OnStop 先于 DB 的 OnStop 执行。
func (s *Service) Shutdown(ctx context.Context) error {
	return s.runtime.WaitIdle(ctx)
}

func (s *Service) worker(ctx context.Context, workerID int, pollInterval time.Duration) {
	timer := time.NewTimer(0)
	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
		}

		// 获取并发槽（阻塞直到有可用容量或 ctx 取消），限制同时进行的 review 数。
		if !s.runtime.acquire(ctx) {
			return
		}
		release := s.runtime.release

		reviews, err := s.reviewRepo.ClaimQueued(ctx, 1)
		if err != nil {
			release()
			if ctx.Err() == nil {
				logx.Error("claim queued review failed", "worker_id", workerID, "error", err)
			}
			timer.Reset(pollInterval)
			continue
		}
		if len(reviews) == 0 {
			release()
			timer.Reset(pollInterval)
			continue
		}

		review := reviews[0]
		req := review.Request
		if req.Provider == "" && req.Repo == "" {
			logx.Error("failed to decode review request", "review_id", review.ID, "error", "empty request")
			s.failureReporter.Report(ctx, review.ID, req, "worker", fmt.Errorf("decode request: empty"), domain.OCRMetadata{})
			release()
			timer.Reset(0)
			continue
		}

		if req.CallbackMode == "scan" {
			logx.Warnf("worker: orphaned scan review from server restart, marking as failed: %s", review.ID)
			s.failureReporter.Report(ctx, review.ID, req, "worker", fmt.Errorf("scan was interrupted by server restart"), domain.OCRMetadata{})
			release()
			timer.Reset(0)
			continue
		}

		provider, ok := s.Providers.Get(req.Provider)
		if !ok {
			err := fmt.Errorf("%w: %q", domain.ErrUnsupportedProvider, req.Provider)
			logx.Errorf("worker: provider not found: requested=%q review_id=%s", req.Provider, review.ID)
			s.failureReporter.Report(ctx, review.ID, req, "worker", err, domain.OCRMetadata{})
			release()
			timer.Reset(0)
			continue
		}
		s.runtime.inFlight.Add(1)
		async.GoSafe(func() {
			defer s.runtime.inFlight.Done()
			defer release()
			// 执行期心跳：长评审期间定期刷新 updated_at（见 startHeartbeat）。
			stopHeartbeat := s.startHeartbeat(review.ID, s.heartbeatInterval)
			defer stopHeartbeat()

			// Create progress tracker for this review
			var progressReporter progress.ProgressReporter
			if s.progressManager != nil {
				tracker := s.progressManager.CreateTracker(review.ID, review.ID, 1)
				progressReporter = tracker
				defer s.progressManager.RemoveTracker(review.ID)
			}

			if _, err := s.Executor.Execute(ctx, review.ID, req, provider, progressReporter); err != nil && ctx.Err() == nil {
				logx.Error("async review failed", "worker_id", workerID, "review_id", review.ID, "error", err)
				// domain.Report error to progress tracker
				if progressReporter != nil {
					if tracker, ok := progressReporter.(*progress.ProgressTracker); ok {
						tracker.SetError(err.Error())
					}
				}
			}
		})
		timer.Reset(0)
	}
}
