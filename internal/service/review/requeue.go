package review

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/yi-nology/code-review-service/internal/domain/review"
	"github.com/yi-nology/code-review-service/internal/repository/repoerr"

	"github.com/oekit/logx"
)

// Requeue 把已结束（failed / success / cancelled）的 review 重新置为 queued 状态。
//
// PR-15 拆分：从 service.go 移出，独立成文件。
//
// 注意：
//   - 当前实现拒绝 status=running 的 review（由外层 executor.CancelExecution 触发后再调用）。
//   - 会先清掉该 review 的 active session，避免「already has an active session」冲突。
func (s *Service) Requeue(ctx context.Context, reviewID string) (domain.Response, error) {
	// Cancel any orphaned active sessions before re-queuing to avoid
	// "already has an active session" conflicts on the next execution.
	if s.sessionRepo != nil {
		if err := s.sessionRepo.CancelActiveByReviewID(ctx, reviewID); err != nil {
			logx.Warnf("requeue: failed to cancel active sessions for %s: %v", reviewID, err)
		}
	}

	var resultResp domain.Response
	err := s.reviewRepo.UpdateInTransaction(ctx, reviewID, func(review *domain.Record) error {
		switch review.Status {
		case StatusQueued, statusRunning:
			return fmt.Errorf("%w: review is %s, only failed or completed reviews can be re-run", ErrInvalidState, review.Status)
		}

		resp := review.Response
		// 保留 OCR session_id（OCR v1.7.6+ resume 支持）：
		// retry 时 executor 会读 response.report.ocr.session_id 传 --resume 复用 checkpoint。
		// 其它 Report 字段（findings/tool_calls 等）清空，避免展示陈旧结果。
		resumeSessionID := resp.Report.OCR.SessionID
		resp.Status = StatusQueued
		resp.Summary = "review re-queued"
		resp.Gate = domain.PendingGate()
		resp.Error = ""
		resp.Findings = nil
		resp.Report = domain.Report{}
		if resumeSessionID != "" {
			resp.Report.OCR.SessionID = resumeSessionID
		}
		resp.Analysis = domain.Analysis{}
		resp.History = append(resp.History, domain.Event{
			Stage:     stageQueued,
			Status:    statusCompleted,
			Message:   "评审已由用户重新入队",
			CreatedAt: time.Now().UTC(),
		})

		review.Status = StatusQueued
		review.Response = resp
		review.UpdatedAt = time.Now().UTC()
		resultResp = resp
		return nil
	})
	if err != nil {
		if errors.Is(err, repoerr.ErrNotFound) {
			return domain.Response{}, fmt.Errorf("%w: %s", ErrNotFound, reviewID)
		}
		return domain.Response{}, err
	}
	return resultResp, nil
}

// Cancel 把 queued/running review 标记为 cancelled，并通知 in-flight scan goroutine 退出。
//
// PR-15 拆分。
//
// 行为：
//   - 同步触发 scanCancels[reviewID] 的 cancel func，scan goroutine 收到 ctx.Done() 后退出
//   - DB 状态置为 failed / Summary = "review cancelled by user"
func (s *Service) Cancel(ctx context.Context, reviewID string) (domain.Response, error) {
	if cancelFn, ok := s.runtime.scanCancels.LoadAndDelete(reviewID); ok {
		if fn, ok := cancelFn.(context.CancelFunc); ok {
			fn()
		}
	}

	var resultResp domain.Response
	err := s.reviewRepo.UpdateInTransaction(ctx, reviewID, func(review *domain.Record) error {
		if review.Status != StatusQueued && review.Status != statusRunning {
			return fmt.Errorf("%w: only queued or running reviews can be cancelled (current: %s)", ErrInvalidState, review.Status)
		}

		resp := review.Response
		resp.Status = statusFailed
		// Summary 是 API 字段（MCP 工具输出给 agent 用），保持机器可读的英文；
		// 门禁原因与审计消息只面向控制台，直接写中文。
		resp.Summary = "review cancelled by user"
		resp.Gate = domain.ErrorGate("评审已取消")
		resp.History = append(resp.History, domain.Event{
			Stage:     stageCancelled,
			Status:    statusFailed,
			Message:   "评审已由用户取消",
			CreatedAt: time.Now().UTC(),
		})

		review.Status = statusFailed
		review.Response = resp
		review.UpdatedAt = time.Now().UTC()
		resultResp = resp
		return nil
	})
	if err != nil {
		if errors.Is(err, repoerr.ErrNotFound) {
			return domain.Response{}, fmt.Errorf("%w: %s", ErrNotFound, reviewID)
		}
		return domain.Response{}, err
	}
	return resultResp, nil
}
