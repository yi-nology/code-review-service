package review

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/yi-nology/code-review-service/internal/repository/postgres/models"

	"gorm.io/datatypes"
)

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
			Status:    StatusEventState(response.Status),
			Message:   StatusEventMessage(response.Status),
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

// StatusEventState maps a review status to a progress event state.
func StatusEventState(status string) string {
	if status == "failed" {
		return "failed"
	}
	if status == "running" {
		return "running"
	}
	return "completed"
}

// StatusEventMessage maps a review status to a human-readable progress event message.
func StatusEventMessage(status string) string {
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
