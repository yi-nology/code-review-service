package notification

import (
	"context"
	"strings"

	"github.com/yi-nology/code-review-service/internal/domain/notification"
	"github.com/oekit/logx"
)

// Notifier 是通知编排器，持有所有已配置的渠道，实现 domain.Notifier。
// 单个渠道失败不影响其它渠道（各自独立记日志）。
type Notifier struct {
	channels []Channel
}

func NewNotifier(channels []Channel) *Notifier {
	return &Notifier{channels: channels}
}

// Notify 同步遍历所有渠道推送。executor 通常用 async.GoSafe 异步调用此方法。
func (n *Notifier) Notify(ctx context.Context, event notification.Event) error {
	if len(n.channels) == 0 {
		return nil
	}
	var errs []string
	for _, ch := range n.channels {
		if err := ch.Send(ctx, event); err != nil {
			logx.Warnf("[notification] channel %s send failed (review=%s kind=%s): %v",
				ch.Name(), event.ReviewID, event.Kind, err)
			errs = append(errs, ch.Name()+": "+err.Error())
		}
	}
	if len(errs) > 0 {
		// 返回聚合错误仅用于日志，不阻断主流程（调用方 async.GoSafe 会忽略）。
		return &multiError{msg: strings.Join(errs, "; ")}
	}
	return nil
}

// NoopNotifier 在未配置任何通知渠道时使用，避免 nil 检查散落各处。
type NoopNotifier struct{}

func (NoopNotifier) Notify(_ context.Context, _ notification.Event) error { return nil }

type multiError struct{ msg string }

func (e *multiError) Error() string { return e.msg }
