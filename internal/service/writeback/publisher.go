package writeback

import (
	"context"

	"github.com/yi-nology/code-review-service/internal/domain/review"
)

type WritebackPublisherAdapter struct {
	orchestrator *Orchestrator
}

func NewWritebackPublisherAdapter(orchestrator *Orchestrator) *WritebackPublisherAdapter {
	return &WritebackPublisherAdapter{orchestrator: orchestrator}
}

func (a *WritebackPublisherAdapter) PublishWithDiff(ctx context.Context, req domain.Request, result domain.Result, diff string) (*domain.WritebackResult, error) {
	return a.orchestrator.Publish(ctx, req, result, diff)
}
