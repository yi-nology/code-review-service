package cost_service

import (
	"context"

	"github.com/yi-nology/code-review-service/biz/handler/common"
	"github.com/yi-nology/code-review-service/internal/service/cost"

	"github.com/cloudwego/hertz/pkg/app"
)

var handlerInst *Handler

type Handler struct {
	calc *cost.Calculator
}

func InitHandler(calc *cost.Calculator) {
	handlerInst = &Handler{calc: calc}
}

// ListPricing 列出所有模型单价。
func (h *Handler) listPricing(ctx context.Context, c *app.RequestContext) {
	if h == nil {
		common.Fail(ctx, c, common.BizErr(500, "cost handler not initialized"))
		return
	}
	rows, err := h.calc.ListPricing(ctx)
	if err != nil {
		common.Fail(ctx, c, err)
		return
	}
	common.Success(c, map[string]any{"items": rows})
}

// ListPricing 包级入口（biz/router 生成代码引用）；一行委托到方法，便于测试直接构造 Handler。
func ListPricing(ctx context.Context, c *app.RequestContext) { handlerInst.listPricing(ctx, c) }

// UpsertPricing 创建或更新一条模型单价。
func (h *Handler) upsertPricing(ctx context.Context, c *app.RequestContext) {
	if h == nil {
		common.Fail(ctx, c, common.BizErr(500, "cost handler not initialized"))
		return
	}
	req, ok := common.Bind[cost.Pricing](c)
	if !ok {
		return
	}
	if req.Provider == "" || req.ModelName == "" {
		common.Fail(ctx, c, common.BadRequest("provider 和 model 不能为空"))
		return
	}
	if err := h.calc.SetPricing(ctx, *req); err != nil {
		common.Fail(ctx, c, err)
		return
	}
	common.Success(c, req)
}

// UpsertPricing 包级入口（biz/router 生成代码引用）；一行委托到方法，便于测试直接构造 Handler。
func UpsertPricing(ctx context.Context, c *app.RequestContext) { handlerInst.upsertPricing(ctx, c) }

// DeletePricing 删除一条模型单价。
func (h *Handler) deletePricing(ctx context.Context, c *app.RequestContext) {
	if h == nil {
		common.Fail(ctx, c, common.BizErr(500, "cost handler not initialized"))
		return
	}
	idStr := c.Param("id")
	id, err := common.ParseUintID(idStr)
	if err != nil {
		common.Fail(ctx, c, common.BadRequest("invalid pricing id"))
		return
	}
	if err := h.calc.DeletePricing(ctx, uint(id)); err != nil {
		common.Fail(ctx, c, err)
		return
	}
	common.Success(c, map[string]any{"ok": true})
}

// DeletePricing 包级入口（biz/router 生成代码引用）；一行委托到方法，便于测试直接构造 Handler。
func DeletePricing(ctx context.Context, c *app.RequestContext) { handlerInst.deletePricing(ctx, c) }
