package domain

import "time"

// StatusEventState 把评审状态映射为进度事件状态。
// 真源下沉在 domain：eino executor / review worker / argus 收敛路径共用，
// 避免底层执行器包反向依赖 service/review。
func StatusEventState(status string) string {
	if status == "failed" {
		return "failed"
	}
	if status == "running" {
		return "running"
	}
	return "completed"
}

// StatusEventMessage 把评审状态映射为人类可读的进度事件消息。
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

// NewStatusEvent 用当前 UTC 时间构造一条状态变迁事件。
// 供 WriteReviewState / updateStatus 等多处状态变迁路径共用，保证事件形态一致。
func NewStatusEvent(status string) Event {
	return Event{
		Stage:     status,
		Status:    StatusEventState(status),
		Message:   StatusEventMessage(status),
		CreatedAt: time.Now().UTC(),
	}
}
