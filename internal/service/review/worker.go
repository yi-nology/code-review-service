package review

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/yi-nology/code-review-service/internal/service/progress"
	"github.com/oekit/concurrency/async"
	"github.com/oekit/concurrency/group"
	"github.com/cloudwego/hertz/pkg/common/hlog"
)

func (s *Service) RunWorkers(ctx context.Context, concurrency int, pollInterval, staleAfter time.Duration) {
	if concurrency <= 0 {
		hlog.Info("review workers disabled")
		return
	}
	if pollInterval <= 0 {
		pollInterval = time.Second
	}
	if staleAfter <= 0 {
		staleAfter = 15 * time.Minute
	}

	requeued, err := s.reviewRepo.RequeueStale(ctx, time.Now().UTC().Add(-staleAfter))
	if err != nil {
		hlog.Error("failed to requeue stale reviews", "error", err)
	} else if requeued > 0 {
		hlog.Warn("requeued stale reviews", "count", requeued)
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
			hlog.Error("worker group failed", "error", err)
		}
	})

	async.GoSafe(func() {
		<-ctx.Done()
		s.inFlight.Wait()
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

		reviews, err := s.reviewRepo.ClaimQueued(ctx, 1)
		if err != nil {
			if ctx.Err() == nil {
				hlog.Error("claim queued review failed", "worker_id", workerID, "error", err)
			}
			timer.Reset(pollInterval)
			continue
		}
		if len(reviews) == 0 {
			timer.Reset(pollInterval)
			continue
		}

		review := reviews[0]
		var req Request
		if err := json.Unmarshal(review.Request, &req); err != nil {
			hlog.Error("failed to decode review request", "review_id", review.ReviewID, "error", err)
			s.recordFailure(ctx, review.ReviewID, req, fmt.Errorf("decode request: %w", err))
			timer.Reset(0)
			continue
		}

		provider, ok := s.Providers.Get(req.Provider)
		if !ok {
			err := fmt.Errorf("%w: %q", ErrUnsupportedProvider, req.Provider)
			s.recordFailure(ctx, review.ReviewID, req, err)
			timer.Reset(0)
			continue
		}
		s.inFlight.Add(1)
		async.GoSafe(func() {
			defer s.inFlight.Done()

			// Create progress tracker for this review
			var progressReporter progress.ProgressReporter
			if s.progressManager != nil {
				tracker := s.progressManager.CreateTracker(review.ReviewID, review.ReviewID, 1)
				progressReporter = tracker
				defer s.progressManager.RemoveTracker(review.ReviewID)
			}

			if _, err := s.Executor.Execute(ctx, review.ReviewID, req, provider, progressReporter); err != nil && ctx.Err() == nil {
				hlog.Error("async review failed", "worker_id", workerID, "review_id", review.ReviewID, "error", err)
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
