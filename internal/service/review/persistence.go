package review

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"review-service/internal/repository/postgres/models"

	"github.com/cloudwego/hertz/pkg/common/hlog"
	"gorm.io/datatypes"
)

func (s *Service) appendHistory(ctx context.Context, id, stage, status, message string, details map[string]any) {
	err := s.reviewRepo.UpdateInTransaction(ctx, id, func(review *models.Review) error {
		var resp Response
		if err := json.Unmarshal(review.Response, &resp); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}

		resp.History = append(resp.History, Event{
			Stage:     stage,
			Status:    status,
			Message:   message,
			Details:   details,
			CreatedAt: time.Now().UTC(),
		})

		review.Response = mustMarshal(resp)
		review.UpdatedAt = time.Now().UTC()
		return nil
	})
	if err != nil {
		hlog.Warn("failed to append review history", "review_id", id, "error", err)
	}
}

func (s *Service) updateStatus(ctx context.Context, id, status string) error {
	return s.reviewRepo.UpdateInTransaction(ctx, id, func(review *models.Review) error {
		var resp Response
		if err := json.Unmarshal(review.Response, &resp); err != nil {
			return fmt.Errorf("decode review response: %w", err)
		}

		previousStatus := resp.Status
		previousHistory := append([]Event(nil), resp.History...)

		resp.Status = status
		resp.Summary = "review " + status
		resp.History = previousHistory

		if status != previousStatus {
			resp.History = append(resp.History, Event{
				Stage:     status,
				Status:    statusEventState(status),
				Message:   statusEventMessage(status),
				CreatedAt: time.Now().UTC(),
			})
		}

		review.Status = status
		review.Response = mustMarshal(resp)
		review.UpdatedAt = time.Now().UTC()
		return nil
	})
}

func (s *Service) updateWithRawOCR(ctx context.Context, id string, req Request, response Response, rawOCR json.RawMessage) error {
	review, err := s.reviewRepo.GetByReviewID(ctx, id)
	if err != nil {
		return err
	}
	if review == nil {
		review = &models.Review{
			ReviewID: id,
			Provider: req.Provider,
			Repo:     req.Repo,
			Request:  mustMarshal(req),
			Status:   response.Status,
		}
	}

	var previousResponse Response
	if err := json.Unmarshal(review.Response, &previousResponse); err != nil {
		return fmt.Errorf("decode previous response: %w", err)
	}

	previousStatus := previousResponse.Status
	previousHistory := append([]Event(nil), previousResponse.History...)

	if len(response.History) == 0 {
		response.History = previousHistory
	}
	if response.Status != "" && response.Status != previousStatus {
		response.History = append(response.History, Event{
			Stage:     response.Status,
			Status:    statusEventState(response.Status),
			Message:   statusEventMessage(response.Status),
			CreatedAt: time.Now().UTC(),
		})
	}

	review.Status = response.Status
	review.Response = mustMarshal(response)
	if len(rawOCR) > 0 {
		review.RawOCR = datatypes.JSON(rawOCR)
	}
	review.UpdatedAt = time.Now().UTC()
	return s.reviewRepo.Save(ctx, review)
}

func (s *Service) updateRequestSHAs(ctx context.Context, id string, req Request) {
	review, err := s.reviewRepo.GetByReviewID(ctx, id)
	if err != nil || review == nil {
		return
	}
	review.Request = mustMarshal(req)
	review.UpdatedAt = time.Now().UTC()
	if err := s.reviewRepo.Save(ctx, review); err != nil {
		hlog.Warn("failed to update request SHAs", "review_id", id, "error", err)
	}
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
