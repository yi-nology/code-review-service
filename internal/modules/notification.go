package modules

import (
	"github.com/yi-nology/code-review-service/internal/config"
	"github.com/yi-nology/code-review-service/internal/repository"
	svcnotif "github.com/yi-nology/code-review-service/internal/service/notification"
	reviewdomain "github.com/yi-nology/code-review-service/internal/service/review/domain"

	"go.uber.org/fx"
)

// NotificationModule 装配通知子系统：站内渠道（DB repo）+ 外部 webhook 渠道。
// 仿 writeback 模式：fx.Provide 构造 Notifier，由 executor 通过 domain.Notifier 接口注入。
var NotificationModule = fx.Options(
	fx.Provide(NewNotifier),
)

// NewNotifier 根据配置构造通知编排器（实现 domain.Notifier）。
// 渠道在构造时显式组装：站内（DB）+ 各外部 webhook（飞书/企微/钉钉）。
// 无任何渠道时返回 NoopNotifier，避免 executor 做 nil 检查。
func NewNotifier(cfg *config.Config, repos *repository.RepoContainer) reviewdomain.Notifier {
	var channels []svcnotif.Channel

	// 站内通知默认开启。
	if !cfg.Notification.InAppDisabled {
		channels = append(channels, svcnotif.NewInAppChannel(repos.Notification))
	}
	// 外部 webhook 渠道。
	for _, wc := range cfg.Notification.Webhooks {
		if wc.URL == "" {
			continue
		}
		channels = append(channels, svcnotif.NewWebhookChannel(wc))
	}

	if len(channels) == 0 {
		return svcnotif.NoopNotifier{}
	}
	return svcnotif.NewNotifier(channels)
}
