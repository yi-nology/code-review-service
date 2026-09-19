package review

import (
	"github.com/yi-nology/code-review-service/internal/service/review/domain"
)

// StatusEventState/StatusEventMessage 的真源在 domain（status_event.go），
// 包内保留别名供存量调用点使用；评审终态写入统一走 state.go 的 WriteReviewState
//（Service 自身的 updateWithRawOCR 已无调用方，随统一收口删除）。

// StatusEventState maps a review status to a progress event state.
func StatusEventState(status string) string {
	return domain.StatusEventState(status)
}

// StatusEventMessage maps a review status to a human-readable progress event message.
func StatusEventMessage(status string) string {
	return domain.StatusEventMessage(status)
}
