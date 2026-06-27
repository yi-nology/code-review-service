package review

import (
	"context"

	"github.com/yi-nology/code-review-service/internal/service/review/domain"
	"github.com/oekit/logx"
)

func (s *Service) recordFailure(ctx context.Context, id string, req domain.Request, runErr error) {
	response := domain.Response{
		ReviewID: id,
		Status:   "failed",
		Summary:  "review failed",
		Gate:     domain.ErrorGate("review execution failed"),
		Error:    runErr.Error(),
	}
	if err := s.updateWithRawOCR(ctx, id, req, response, nil); err != nil {
		logx.Error("failed to persist review failure", "review_id", id, "error", err)
	}
}
