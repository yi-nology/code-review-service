package cost_service

import (
	"context"
	"strconv"

	"github.com/yi-nology/code-review-service/biz/handler/common"
	"github.com/yi-nology/code-review-service/internal/repository/postgres/models"
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
func ListPricing(ctx context.Context, c *app.RequestContext) {
	if handlerInst == nil {
		common.Fail(c, common.BizErr(500, "cost handler not initialized"))
		return
	}
	rows, err := handlerInst.calc.ListPricing(ctx)
	if err != nil {
		common.Fail(c, err)
		return
	}
	common.Success(c, map[string]any{"items": rows})
}

// UpsertPricing 创建或更新一条模型单价。
func UpsertPricing(ctx context.Context, c *app.RequestContext) {
	if handlerInst == nil {
		common.Fail(c, common.BizErr(500, "cost handler not initialized"))
		return
	}
	req, ok := common.Bind[models.LLMModelPricing](c)
	if !ok {
		return
	}
	if req.Provider == "" || req.ModelName == "" {
		common.Fail(c, common.BadRequest("provider 和 model 不能为空"))
		return
	}
	if err := handlerInst.calc.SetPricing(ctx, *req); err != nil {
		common.Fail(c, err)
		return
	}
	common.Success(c, req)
}

// DeletePricing 删除一条模型单价。
func DeletePricing(ctx context.Context, c *app.RequestContext) {
	if handlerInst == nil {
		common.Fail(c, common.BizErr(500, "cost handler not initialized"))
		return
	}
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		common.Fail(c, common.BadRequest("invalid pricing id"))
		return
	}
	if err := handlerInst.calc.DeletePricing(ctx, uint(id)); err != nil {
		common.Fail(c, err)
		return
	}
	common.Success(c, map[string]any{"ok": true})
}
