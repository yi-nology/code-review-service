package review

import (
	"context"
	"errors"
	"time"

	"github.com/yi-nology/code-review-service/internal/repository/repoerr"
	"github.com/yi-nology/code-review-service/internal/service/review/domain"
)

// updateWithRawOCR 更新评审的 Response/RawOCR（#6 keystone：domain.Record 强类型，
// 不再做 JSONB marshal/unmarshal 往返）。
func (s *Service) updateWithRawOCR(ctx context.Context, id string, req domain.Request, response domain.Response, rawOCR []byte) error {
	review, err := s.reviewRepo.GetByReviewID(ctx, id)
	if err != nil && !errors.Is(err, repoerr.ErrNotFound) {
		return err
	}
	if review == nil {
		review = &domain.Record{
			ID:       id,
			Provider: req.Provider,
			Repo:     req.Repo,
			Request:  req,
			Status:   response.Status,
		}
	}

	previousResponse := review.Response
	previousStatus := previousResponse.Status
	previousHistory := append([]domain.Event(nil), previousResponse.History...)

	if len(response.History) == 0 {
		response.History = previousHistory
	}
	if response.Status != "" && response.Status != previousStatus {
		response.History = append(response.History, domain.Event{
			Stage:     response.Status,
			Status:    StatusEventState(response.Status),
			Message:   StatusEventMessage(response.Status),
			CreatedAt: time.Now().UTC(),
		})
	}

	review.Status = response.Status
	review.Response = response
	if len(rawOCR) > 0 {
		review.RawOCR = rawOCR
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
