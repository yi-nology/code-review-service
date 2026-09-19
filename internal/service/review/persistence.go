package review

import (
	"context"

	"github.com/yi-nology/code-review-service/internal/service/review/domain"
)

// updateWithRawOCR 更新评审的 Response/RawOCR（#6 keystone：domain.Record 强类型，
// 不再做 JSONB marshal/unmarshal 往返）。实现统一收敛到 state.go 的 WriteReviewState
// （此前本包与 internal/eino 各持一份且语义分叉）。
func (s *Service) updateWithRawOCR(ctx context.Context, id string, req domain.Request, response domain.Response, rawOCR []byte) error {
	return WriteReviewState(ctx, s.reviewRepo, id, req, response, rawOCR)
}

// StatusEventState maps a review status to a progress event state.
// 真源在 domain（供 eino/worker/收敛路径共用）；此处保留别名以减少存量调用点改动。
func StatusEventState(status string) string {
	return domain.StatusEventState(status)
}

// StatusEventMessage maps a review status to a human-readable progress event message.
func StatusEventMessage(status string) string {
	return domain.StatusEventMessage(status)
}
