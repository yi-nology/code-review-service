// Package notification 实现评审事件通知（站内消息 + 外部 webhook）。
//
// 设计仿 writeback：Notifier（Orchestrator）持有一组 Channel（inapp/feishu/wecom/...），
// 按配置触发。executor 通过 domain.Notifier 接口注入，用 async.GoSafe 做 fire-and-forget。
package notification

import (
	"context"

	"github.com/yi-nology/code-review-service/internal/domain/notification"
)

// Channel 是一个通知渠道（站内 / 飞书 / 企业微信 / ...）。
type Channel interface {
	// Name 返回渠道标识（inapp / feishu / wecom）。
	Name() string
	// Send 推送一条通知。返回 error 仅用于日志，不影响其它渠道。
	Send(ctx context.Context, event notification.Event) error
}

// shouldNotify 按 channel 配置的 On 条件判断是否推送。
// on 为空表示所有事件都推送。
func shouldNotify(on []string, kind notification.Kind) bool {
	if len(on) == 0 {
		return true
	}
	k := string(kind)
	for _, o := range on {
		if o == k {
			return true
		}
	}
	return false
}
