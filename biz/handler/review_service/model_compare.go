package review_service

import (
	"context"
	"strings"

	"github.com/yi-nology/code-review-service/biz/handler/common"
	"github.com/yi-nology/code-review-service/internal/service/review"

	"github.com/cloudwego/hertz/pkg/app"
)

func CompareReviewModels(ctx context.Context, c *app.RequestContext) {
	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		common.Fail(c, common.BadRequest("review id 不能为空"))
		return
	}
	req, ok := common.Bind[review.ModelCompareOptions](c)
	if !ok {
		return
	}
	if len(req.Candidates) == 0 {
		common.Fail(c, common.BadRequest("candidates 不能为空"))
		return
	}
	if len(req.Candidates) > 4 {
		common.Fail(c, common.BadRequest("candidates 最多支持 4 个"))
		return
	}
	result, found, err := handlerInst.reviewSvc.CompareModels(ctx, id, *req)
	if err != nil {
		common.Fail(c, err)
		return
	}
	if !found {
		common.Fail(c, common.NotFound("review not found"))
		return
	}
	common.Success(c, result)
}
