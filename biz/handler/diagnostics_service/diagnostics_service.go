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

func Snapshot(ctx context.Context, c *app.RequestContext) {
	if handlerInst == nil || handlerInst.svc == nil {
		common.Fail(c, common.BizErr(500, "diagnostics handler not initialized"))
		return
	}
	req, ok := common.Bind[snapshotReq](c)
	if !ok {
		return
	}
	snapshot, err := handlerInst.svc.Snapshot(ctx, diagnostics.SnapshotOptions{
		Hours:    req.Hours,
		Limit:    req.Limit,
		Provider: strings.TrimSpace(req.Provider),
		Status:   strings.TrimSpace(req.Status),
	})
	if err != nil {
		common.Fail(c, err)
		return
	}
	common.Success(c, snapshot)
}
