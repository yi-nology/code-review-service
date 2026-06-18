package executor

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/yi-nology/code-review-service/internal/repository/postgres/models"
	"github.com/yi-nology/code-review-service/internal/service/review"

	"github.com/cloudwego/hertz/pkg/common/hlog"
	"gorm.io/datatypes"
)

func (s *Service) appendHistory(ctx context.Context, id, stage, status, message string, details map[string]any) {
	err := s.reviewRepo.UpdateInTransaction(ctx, id, func(r *models.Review) error {
		var resp review.Response
		if err := json.Unmarshal(r.Response, &resp); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
		resp.History = append(resp.History, review.Event{
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
		hlog.Warn("failed to append review history", "review_id", id, "error", err)
	}
}

func (s *Service) updateStatus(ctx context.Context, id, status string) error {
	return s.reviewRepo.UpdateInTransaction(ctx, id, func(r *models.Review) error {
		var resp review.Response
		if err := json.Unmarshal(r.Response, &resp); err != nil {
			return fmt.Errorf("decode review response: %w", err)
		}
		previousStatus := resp.Status
		previousHistory := append([]review.Event(nil), resp.History...)
		resp.Status = status
		resp.Summary = "review " + status
		resp.History = previousHistory
		if status != previousStatus {
			resp.History = append(resp.History, review.Event{
				Stage:     status,
				Status:    statusEventState(status),
				Message:   statusEventMessage(status),
				CreatedAt: time.Now().UTC(),
			})
		}
		r.Status = status
		r.Response = review.MustMarshal(resp)
		r.UpdatedAt = time.Now().UTC()
		return nil
	})
}

func (s *Service) updateWithRawOCR(ctx context.Context, id string, req review.Request, response review.Response, rawOCR json.RawMessage) error {
	r, err := s.reviewRepo.GetByReviewID(ctx, id)
	if err != nil {
		return err
	}
	if r == nil {
		r = &models.Review{
			ReviewID: id,
			Provider: req.Provider,
			Repo:     req.Repo,
			Request:  review.MustMarshal(req),
			Status:   response.Status,
		}
	}

	var previousResponse review.Response
	if err := json.Unmarshal(r.Response, &previousResponse); err != nil {
		return fmt.Errorf("decode previous response: %w", err)
	}
	previousStatus := previousResponse.Status
	previousHistory := append([]review.Event(nil), previousResponse.History...)
	if len(response.History) == 0 {
		response.History = previousHistory
	}
	if response.Status != "" && response.Status != previousStatus {
		response.History = append(response.History, review.Event{
			Stage:     response.Status,
			Status:    statusEventState(response.Status),
			Message:   statusEventMessage(response.Status),
			CreatedAt: time.Now().UTC(),
		})
	}

	r.Status = response.Status
	r.Response = review.MustMarshal(response)
	if len(rawOCR) > 0 {
		r.RawOCR = datatypes.JSON(rawOCR)
	}
	r.UpdatedAt = time.Now().UTC()
	return s.reviewRepo.Save(ctx, r)
}

func (s *Service) updateRequestSHAs(ctx context.Context, id string, req review.Request) {
	r, err := s.reviewRepo.GetByReviewID(ctx, id)
	if err != nil || r == nil {
		return
	}
	r.Request = review.MustMarshal(req)
	r.UpdatedAt = time.Now().UTC()
	if err := s.reviewRepo.Save(ctx, r); err != nil {
		hlog.Warn("failed to update request SHAs", "review_id", id, "error", err)
	}
}

func (s *Service) updateWithOCRInput(ctx context.Context, id string, ocrInput review.OCRInput) error {
	r, err := s.reviewRepo.GetByReviewID(ctx, id)
	if err != nil {
		return err
	}
	if r == nil {
		return fmt.Errorf("review not found: %s", id)
	}
	r.RawOCRInput = review.MustMarshal(ocrInput)
	r.UpdatedAt = time.Now().UTC()
	return s.reviewRepo.Save(ctx, r)
}

func statusEventState(status string) string {
	if status == "failed" {
		return "failed"
	}
	if status == "running" {
		return "running"
	}
	return "completed"
}

func statusEventMessage(status string) string {
	switch status {
	case "running":
		return "worker claimed the task and started execution"
	case "success":
		return "review report and quality gate generated"
	case "failed":
		return "review execution failed"
	default:
		return status
	}
}
