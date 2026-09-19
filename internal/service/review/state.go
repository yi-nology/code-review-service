package review

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/yi-nology/code-review-service/internal/repository/repoerr"
	"github.com/yi-nology/code-review-service/internal/service/review/domain"
	"github.com/oekit/logx"
)

// ReviewStateStore 是写入评审终态所需的最小仓储面。
// repository.ReviewRepository 是其超集；测试桩只需实现这四个方法。
type ReviewStateStore interface {
	GetByReviewID(ctx context.Context, reviewID string) (*domain.Record, error)
	Create(ctx context.Context, review *domain.Record) error
	Save(ctx context.Context, review *domain.Record) error
	UpdateInTransaction(ctx context.Context, reviewID string, fn func(review *domain.Record) error) error
}

// WriteReviewState 写入评审终态（唯一的实现，此前 review.Service 与
// internal/eino 各持一份且语义分叉：旧 review 版不写 Score/Grade/Summary、
// 用非事务 Save；旧 eino 版写全但两份都要人肉同步）。语义：
//   - 记录不存在时 Create（带 Response/Score/Grade）；
//   - 存在时事务内合并 history（状态变迁追加事件）并更新
//     Status/Summary/Score/Grade/Response/RawOCR/UpdatedAt。
func WriteReviewState(ctx context.Context, store ReviewStateStore, id string, req domain.Request, response domain.Response, rawOCR []byte) error {
	existing, err := store.GetByReviewID(ctx, id)
	if err != nil && !errors.Is(err, repoerr.ErrNotFound) {
		return err
	}
	if existing == nil {
		rec := &domain.Record{
			ID:       id,
			Provider: req.Provider,
			Repo:     req.Repo,
			Request:  req,
			Status:   response.Status,
			Summary:  response.Summary,
			Response: response,
			Score:    response.Report.Score.Value,
			Grade:    response.Report.Score.Grade,
		}
		if len(rawOCR) > 0 {
			rec.RawOCR = rawOCR
		}
		rec.UpdatedAt = time.Now().UTC()
		return store.Create(ctx, rec)
	}

	return store.UpdateInTransaction(ctx, id, func(r *domain.Record) error {
		previousResponse := r.Response
		previousStatus := previousResponse.Status
		previousHistory := append([]domain.Event(nil), previousResponse.History...)
		if len(response.History) == 0 {
			response.History = previousHistory
		}
		if response.Status != "" && response.Status != previousStatus {
			response.History = append(response.History, domain.NewStatusEvent(response.Status))
		}

		r.Status = response.Status
		r.Summary = response.Summary
		r.Score = response.Report.Score.Value
		r.Grade = response.Report.Score.Grade
		r.Response = response
		if len(rawOCR) > 0 {
			r.RawOCR = rawOCR
		}
		r.UpdatedAt = time.Now().UTC()
		return nil
	})
}

// AppendHistoryEvent 向评审历史追加一条事件（事务内）。供 FailureReporter 与
// 执行器共享；失败仅记日志（历史是辅助信息，不应让主流程失败）。
func AppendHistoryEvent(ctx context.Context, store ReviewStateStore, id string, event domain.Event) {
	if store == nil {
		return
	}
	err := store.UpdateInTransaction(ctx, id, func(r *domain.Record) error {
		resp := r.Response
		resp.History = append(resp.History, event)
		r.Response = resp
		r.UpdatedAt = time.Now().UTC()
		return nil
	})
	if err != nil {
		logx.Warn("failed to append history event", "review_id", id, "stage", event.Stage, "error", err)
	}
}

// WriteOCRInput 把 OCR 执行入参快照落库（RawOCRInput），供 resume/审计共用。
// 此前 eino 与 scan 路径各持一份等价实现。记录不存在时返回 ErrNotFound。
func WriteOCRInput(ctx context.Context, store ReviewStateStore, id string, ocrInput domain.OCRInput) error {
	raw, err := json.Marshal(ocrInput)
	if err != nil {
		return fmt.Errorf("marshal ocr input: %w", err)
	}
	return store.UpdateInTransaction(ctx, id, func(r *domain.Record) error {
		r.RawOCRInput = raw
		r.UpdatedAt = time.Now().UTC()
		return nil
	})
}
