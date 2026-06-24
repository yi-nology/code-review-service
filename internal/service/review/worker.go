package review

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/yi-nology/code-review-service/internal/service/progress"
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

	// 并发槽：限制同时进行的 review 数 = worker concurrency。
	// 修复 #5.3/#17：原 worker spawn goroutine 后立即再 claim，并发不受 worker 数限制，
	// webhook 涌入时瞬间起几十个 OCR 进程压垮 LLM endpoint（雪崩根因）。
	if s.concurrencySem == nil {
		s.concurrencySem = make(chan struct{}, concurrency)
		logx.Infof("review concurrency limit=%d（同时进行的 review 上限）", concurrency)
	}

	// 启动时：重排所有 running 评审（服务重启后全是孤儿，必须重新处理）
	orphaned, err := s.reviewRepo.RequeueStale(ctx, time.Now().UTC().Add(1*time.Hour))
	if err != nil {
		logx.Error("failed to requeue orphaned reviews on startup", "error", err)
	} else if orphaned > 0 {
		logx.Warnf("requeued %d orphaned running reviews from previous instance", orphaned)
	}

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

	async.GoSafe(func() {
		<-ctx.Done()
		s.inFlight.Wait()
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
				n, err := s.reviewRepo.RequeueStale(ctx, time.Now().UTC().Add(-staleAfter))
				if err != nil {
					logx.Error("periodic requeue stale failed", "error", err)
				} else if n > 0 {
					logx.Warnf("periodic requeue: %d stale reviews re-queued", n)
				}
			}
		}
	})
}

func (s *Service) WaitInFlight() {
	s.inFlight.Wait()
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

		// 获取并发槽（阻塞直到有可用容量），限制同时进行的 review 数。
		select {
		case s.concurrencySem <- struct{}{}:
		case <-ctx.Done():
			return
		}
		release := func() { <-s.concurrencySem }

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
		var req Request
		if err := json.Unmarshal(review.Request, &req); err != nil {
			logx.Error("failed to decode review request", "review_id", review.ReviewID, "error", err)
			s.recordFailure(ctx, review.ReviewID, req, fmt.Errorf("decode request: %w", err))
			release()
			timer.Reset(0)
			continue
		}

		provider, ok := s.Providers.Get(req.Provider)
		if !ok {
			err := fmt.Errorf("%w: %q", ErrUnsupportedProvider, req.Provider)
			logx.Errorf("worker: provider not found: requested=%q review_id=%s", req.Provider, review.ReviewID)
			s.recordFailure(ctx, review.ReviewID, req, err)
			release()
			timer.Reset(0)
			continue
		}
		s.inFlight.Add(1)
		async.GoSafe(func() {
			defer s.inFlight.Done()
			defer release()

			// Create progress tracker for this review
			var progressReporter progress.ProgressReporter
			if s.progressManager != nil {
				tracker := s.progressManager.CreateTracker(review.ReviewID, review.ReviewID, 1)
				progressReporter = tracker
				defer s.progressManager.RemoveTracker(review.ReviewID)
			}

			if _, err := s.Executor.Execute(ctx, review.ReviewID, req, provider, progressReporter); err != nil && ctx.Err() == nil {
				logx.Error("async review failed", "worker_id", workerID, "review_id", review.ReviewID, "error", err)
				// Report error to progress tracker
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
