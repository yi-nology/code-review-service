package executor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/yi-nology/code-review-service/internal/repository/repoerr"
	"github.com/yi-nology/code-review-service/internal/service/review"
	"github.com/yi-nology/code-review-service/internal/service/review/domain"

	"github.com/oekit/logx"
)

// appendHistory 追加一条评审历史事件（#6 keystone：domain.Record 强类型，无 JSONB 往返）。
func (s *Service) appendHistory(ctx context.Context, id, stage, status, message string, details map[string]any) {
	err := s.reviewRepo.UpdateInTransaction(ctx, id, func(r *domain.Record) error {
		resp := r.Response
		resp.History = append(resp.History, domain.Event{
			Stage:     stage,
			Status:    status,
			Message:   message,
			Details:   details,
			CreatedAt: time.Now().UTC(),
		})
		r.Response = resp
		r.UpdatedAt = time.Now().UTC()
		return nil
	})
	if err != nil {
		logx.Warn("failed to append review history", "review_id", id, "error", err)
	}
}

func (s *Service) updateStatus(ctx context.Context, id, status string) error {
	return s.reviewRepo.UpdateInTransaction(ctx, id, func(r *domain.Record) error {
		resp := r.Response
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
		r.Response = resp
		r.UpdatedAt = time.Now().UTC()
		return nil
	})
}

// updateWithRawOCR 写入 review 最终态 + raw OCR payload（单事务读-改-写，PR-20）。
func (s *Service) updateWithRawOCR(ctx context.Context, id string, req domain.Request, response domain.Response, rawOCR json.RawMessage) error {
	existing, err := s.reviewRepo.GetByReviewID(ctx, id)
	if err != nil && !errors.Is(err, repoerr.ErrNotFound) {
		return err
	}
	if existing == nil {
		newReview := &domain.Record{
			ID:       id,
			Provider: req.Provider,
			Repo:     req.Repo,
			Request:  req,
			Status:   response.Status,
			Response: response,
			Score:    response.Report.Score.Value,
			Grade:    response.Report.Score.Grade,
		}
		if len(rawOCR) > 0 {
			newReview.RawOCR = rawOCR
		}
		newReview.UpdatedAt = time.Now().UTC()
		return s.reviewRepo.Create(ctx, newReview)
	}

	return s.reviewRepo.UpdateInTransaction(ctx, id, func(r *domain.Record) error {
		previousResponse := r.Response
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
		r.Response = response
		if len(rawOCR) > 0 {
			r.RawOCR = rawOCR
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
	r.Request = req
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
	rawOCRInputJSON, err := json.Marshal(ocrInput)
	if err != nil {
		return fmt.Errorf("marshal ocr input: %w", err)
	}
	r.RawOCRInput = rawOCRInputJSON
	r.UpdatedAt = time.Now().UTC()
	return s.reviewRepo.Save(ctx, r)
}
