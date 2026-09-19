package diagnostics_service

import (
	"context"
	"strings"

	"github.com/yi-nology/code-review-service/biz/handler/common"
	"github.com/yi-nology/code-review-service/internal/service/diagnostics"

	"github.com/cloudwego/hertz/pkg/app"
)

var handlerInst *Handler

type Handler struct {
	svc *diagnostics.Service
}

func InitHandler(svc *diagnostics.Service) {
	handlerInst = &Handler{svc: svc}
}

type snapshotReq struct {
	Hours    int    `query:"hours"`
	Limit    int    `query:"limit"`
	Provider string `query:"provider"`
	Status   string `query:"status"`
}

func (h *Handler) snapshot(ctx context.Context, c *app.RequestContext) {
	if h == nil || h.svc == nil {
		common.Fail(ctx, c, common.BizErr(500, "diagnostics handler not initialized"))
		return
	}
	req, ok := common.Bind[snapshotReq](c)
	if !ok {
		return
	}
	snapshot, err := h.svc.Snapshot(ctx, diagnostics.SnapshotOptions{
		Hours:    req.Hours,
		Limit:    req.Limit,
		Provider: strings.TrimSpace(req.Provider),
		Status:   strings.TrimSpace(req.Status),
	})
	if err != nil {
		common.Fail(ctx, c, err)
		return
	}
	common.Success(c, snapshot)
}

// Snapshot 包级入口（biz/router 生成代码引用）；一行委托到方法，便于测试直接构造 Handler。
func Snapshot(ctx context.Context, c *app.RequestContext) { handlerInst.snapshot(ctx, c) }
