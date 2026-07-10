package notification_service

import (
	"context"
	"strconv"

	"github.com/yi-nology/code-review-service/biz/handler/common"
	"github.com/yi-nology/code-review-service/internal/domain/notification"
	"github.com/yi-nology/code-review-service/internal/repository"

	"github.com/cloudwego/hertz/pkg/app"
)

var handlerInst *Handler

// Handler 暴露站内通知的查询/已读操作。通知的「生成」由 executor 在评审结束时
// 通过 notification.Notifier 触发（写 DB），此处仅负责前端读取。
type Handler struct {
	repo repository.NotificationRepository
}

func InitHandler(repo repository.NotificationRepository) {
	handlerInst = &Handler{repo: repo}
}

func (h *Handler) list(ctx context.Context, c *app.RequestContext) {
	if h == nil || h.repo == nil {
		common.Fail(c, common.BizErr(500, "notification handler not initialized"))
		return
	}
	var opts notification.ListOptions
	if v := c.QueryArgs().QueryString(); len(v) > 0 {
		// 支持 ?only_unread=true
		if string(c.QueryArgs().Peek("only_unread")) == "true" {
			opts.OnlyUnread = true
		}
	}
	if limit, err := strconv.Atoi(string(c.QueryArgs().Peek("limit"))); err == nil && limit > 0 {
		opts.Limit = limit
	}
	if offset, err := strconv.Atoi(string(c.QueryArgs().Peek("offset"))); err == nil && offset >= 0 {
		opts.Offset = offset
	}
	items, total, err := h.repo.List(ctx, opts)
	if err != nil {
		common.Fail(c, err)
		return
	}
	common.Success(c, map[string]any{"items": items, "total": total})
}

func (h *Handler) unreadCount(ctx context.Context, c *app.RequestContext) {
	if h == nil || h.repo == nil {
		common.Fail(c, common.BizErr(500, "notification handler not initialized"))
		return
	}
	n, err := h.repo.UnreadCount(ctx)
	if err != nil {
		common.Fail(c, err)
		return
	}
	common.Success(c, map[string]any{"unread": n})
}

func (h *Handler) markRead(ctx context.Context, c *app.RequestContext) {
	if h == nil || h.repo == nil {
		common.Fail(c, common.BizErr(500, "notification handler not initialized"))
		return
	}
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		common.Fail(c, common.BadRequest("invalid notification id"))
		return
	}
	if err := h.repo.MarkRead(ctx, uint(id)); err != nil {
		common.Fail(c, err)
		return
	}
	common.Success(c, map[string]any{"ok": true})
}

func (h *Handler) markAllRead(ctx context.Context, c *app.RequestContext) {
	if h == nil || h.repo == nil {
		common.Fail(c, common.BizErr(500, "notification handler not initialized"))
		return
	}
	if err := h.repo.MarkAllRead(ctx); err != nil {
		common.Fail(c, err)
		return
	}
	common.Success(c, map[string]any{"ok": true})
}

// 包级 handler 函数（供 server.go 路由注册引用）
func List(ctx context.Context, c *app.RequestContext)        { handlerInst.list(ctx, c) }
func UnreadCount(ctx context.Context, c *app.RequestContext) { handlerInst.unreadCount(ctx, c) }
func MarkRead(ctx context.Context, c *app.RequestContext)    { handlerInst.markRead(ctx, c) }
func MarkAllRead(ctx context.Context, c *app.RequestContext) { handlerInst.markAllRead(ctx, c) }
