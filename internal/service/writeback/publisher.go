package writeback

import (
	"context"

	"github.com/yi-nology/code-review-service/internal/service/review"
)

type WritebackPublisherAdapter struct {
	orchestrator *Orchestrator
}

func NewWritebackPublisherAdapter(orchestrator *Orchestrator) *WritebackPublisherAdapter {
	return &WritebackPublisherAdapter{orchestrator: orchestrator}
}

func (a *WritebackPublisherAdapter) PublishWithDiff(ctx context.Context, req review.Request, result review.Result, diff string) (*review.WritebackResult, error) {
	return a.orchestrator.Publish(ctx, req, result, diff)
}
