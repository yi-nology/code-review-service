package executor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/yi-nology/code-review-service/internal/repository/postgres/models"
	"github.com/yi-nology/code-review-service/internal/repository/repoerr"
	"github.com/yi-nology/code-review-service/internal/service/review"
	"github.com/yi-nology/code-review-service/internal/service/review/domain"

	"github.com/oekit/logx"
	"gorm.io/datatypes"
)

func (s *Service) appendHistory(ctx context.Context, id, stage, status, message string, details map[string]any) {
	err := s.reviewRepo.UpdateInTransaction(ctx, id, func(r *models.Review) error {
		var resp domain.Response
		if err := json.Unmarshal(r.Response, &resp); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
		resp.History = append(resp.History, domain.Event{
			Stage:     stage,
			Status:    status,
			Message:   message,
			Details:   details,
			CreatedAt: time.Now().UTC(),
		})
		r.Response = review.MustMarshal(resp)
		r.UpdatedAt = time.Now().UTC()
		return nil
	})
	if err != nil {
		logx.Warn("failed to append review history", "review_id", id, "error", err)
	}
}

func (s *Service) updateStatus(ctx context.Context, id, status string) error {
	return s.reviewRepo.UpdateInTransaction(ctx, id, func(r *models.Review) error {
		var resp domain.Response
		if err := json.Unmarshal(r.Response, &resp); err != nil {
			return fmt.Errorf("decode review response: %w", err)
		}
		previousStatus := resp.Status
		previousHistory := append([]domain.Event(nil), resp.History...)
		resp.Status = status
		resp.Summary = "review " + status
		resp.History = previousHistory
		if status != previousStatus {
			resp.History = append(resp.History, domain.Event{
				Stage:     status,
				Status:    review.StatusEventState(status),
				Message:   review.StatusEventMessage(status),
				CreatedAt: time.Now().UTC(),
			})
		}
		r.Status = status
		r.Summary = resp.Summary
		r.Response = review.MustMarshal(resp)
		r.UpdatedAt = time.Now().UTC()
		return nil
	})
}

// updateWithRawOCR 写入 review 最终态 + raw OCR payload。
//
// PR-20 修复：原实现是 GetByReviewID + Save 两步独立调用，
// 读与写不在同一事务，并发场景下可能丢失 update。
// 改为 UpdateInTransaction 包裹整个读-改-写流程：
//   - 单次 tx 内 SELECT ... FOR UPDATE 锁定行
//   - 应用 patch
//   - 单次 UPDATE 落库
// 网络往返从 2 次降为 1 次，并发安全。
func (s *Service) updateWithRawOCR(ctx context.Context, id string, req domain.Request, response domain.Response, rawOCR json.RawMessage) error {
	// 第一次：尝试找现有 review
	existing, err := s.reviewRepo.GetByReviewID(ctx, id)
	if err != nil && !errors.Is(err, repoerr.ErrNotFound) {
		return err
	}
	if existing == nil {
		// 不存在则 Create（独立 tx）
		newReview := &models.Review{
			ReviewID: id,
			Provider: req.Provider,
			Repo:     req.Repo,
			Request:  review.MustMarshal(req),
			Status:   response.Status,
			Response: review.MustMarshal(response),
		}
		newReview.Score = response.Report.Score.Value
		newReview.Grade = response.Report.Score.Grade
		if len(rawOCR) > 0 {
			newReview.RawOCR = datatypes.JSON(rawOCR)
		}
		newReview.UpdatedAt = time.Now().UTC()
		return s.reviewRepo.Create(ctx, newReview)
	}

	// 已存在：合并到单 tx
	return s.reviewRepo.UpdateInTransaction(ctx, id, func(r *models.Review) error {
		var previousResponse domain.Response
		if err := json.Unmarshal(r.Response, &previousResponse); err != nil {
			return fmt.Errorf("decode previous response: %w", err)
		}
		previousStatus := previousResponse.Status
		previousHistory := append([]domain.Event(nil), previousResponse.History...)
		if len(response.History) == 0 {
			response.History = previousHistory
		}
		if response.Status != "" && response.Status != previousStatus {
			response.History = append(response.History, domain.Event{
				Stage:     response.Status,
				Status:    review.StatusEventState(response.Status),
				Message:   review.StatusEventMessage(response.Status),
				CreatedAt: time.Now().UTC(),
			})
		}

		r.Status = response.Status
		r.Summary = response.Summary
		r.Score = response.Report.Score.Value
		r.Grade = response.Report.Score.Grade
		r.Response = review.MustMarshal(response)
		if len(rawOCR) > 0 {
			r.RawOCR = datatypes.JSON(rawOCR)
		}
		r.UpdatedAt = time.Now().UTC()
		return nil
	})
}

func (s *Service) updateRequestSHAs(ctx context.Context, id string, req domain.Request) {
	r, err := s.reviewRepo.GetByReviewID(ctx, id)
	if err != nil {
		return
	}
	r.Request = review.MustMarshal(req)
	r.UpdatedAt = time.Now().UTC()
	if err := s.reviewRepo.Save(ctx, r); err != nil {
		logx.Warn("failed to update request SHAs", "review_id", id, "error", err)
	}
}

func (s *Service) updateWithOCRInput(ctx context.Context, id string, ocrInput domain.OCRInput) error {
	r, err := s.reviewRepo.GetByReviewID(ctx, id)
	if err != nil {
		if errors.Is(err, repoerr.ErrNotFound) {
			return fmt.Errorf("review not found: %s", id)
		}
		return err
	}
	r.RawOCRInput = review.MustMarshal(ocrInput)
	r.UpdatedAt = time.Now().UTC()
	return s.reviewRepo.Save(ctx, r)
}

// StatusEventState and StatusEventMessage are now in review package.
// Use review.StatusEventState / review.StatusEventMessage.
