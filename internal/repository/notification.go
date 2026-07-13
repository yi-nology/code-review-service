package repository

import (
	"context"

	"github.com/yi-nology/code-review-service/internal/domain/notification"
)

// NotificationRepository 管理站内通知的持久化。
type NotificationRepository interface {
	Create(ctx context.Context, item *notification.Item) error
	List(ctx context.Context, opts notification.ListOptions) ([]notification.Item, int64, error)
	// MarkRead 标记单条通知已读。
	MarkRead(ctx context.Context, id uint) error
	// MarkAllRead 标记全部未读通知为已读。
	MarkAllRead(ctx context.Context) error
	// UnreadCount 返回未读通知数。
	UnreadCount(ctx context.Context) (int64, error)
}
