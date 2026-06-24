package review

import (
	"context"

	"github.com/oekit/logx"
)

func (s *Service) recordFailure(ctx context.Context, id string, req Request, runErr error) {
	response := Response{
		ReviewID: id,
		Status:   "failed",
		Summary:  "review failed",
		Gate:     ErrorGate("review execution failed"),
		Error:    runErr.Error(),
	}
	if err := s.updateWithRawOCR(ctx, id, req, response, nil); err != nil {
		logx.Error("failed to persist review failure", "review_id", id, "error", err)
	}
}
