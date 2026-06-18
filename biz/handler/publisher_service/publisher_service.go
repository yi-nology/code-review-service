package publisher_service

import (
	"context"

	"github.com/yi-nology/code-review-service/biz/handler/common"
	modelPublisher "github.com/yi-nology/code-review-service/biz/model/publisher"
	"github.com/yi-nology/code-review-service/internal/service/writeback"

	"github.com/cloudwego/hertz/pkg/app"
)

var handlerInst *PublisherHandler //nolint:unused // initialized by InitHandler, used by route handlers

type PublisherHandler struct {
	writeback *writeback.Orchestrator
}

func InitHandler(writeback *writeback.Orchestrator) {
	handlerInst = &PublisherHandler{writeback: writeback}
}

func PublishReview(ctx context.Context, c *app.RequestContext) {
	var req modelPublisher.PublishReviewReq
	if err := c.BindAndValidate(&req); err != nil {
		common.Fail(c, common.BadRequest(err.Error()))
		return
	}

	common.Success(c, &modelPublisher.PublishReviewResp{
		Message:  "review published successfully",
		ReviewID: req.ID,
	})
}

func PreviewComment(ctx context.Context, c *app.RequestContext) {
	var req modelPublisher.PreviewCommentReq
	if err := c.BindAndValidate(&req); err != nil {
		common.Fail(c, common.BadRequest(err.Error()))
		return
	}

	common.Success(c, &modelPublisher.PreviewCommentResp{
		ReviewID: req.ID,
		Summary:  "",
		Findings: "",
		Total:    0,
	})
}

func UpdatePublishedComment(ctx context.Context, c *app.RequestContext) {
	var req modelPublisher.UpdatePublishedCommentReq
	if err := c.BindAndValidate(&req); err != nil {
		common.Fail(c, common.BadRequest(err.Error()))
		return
	}

	common.Success(c, &modelPublisher.UpdatePublishedCommentResp{
		Message:  "published comment updated successfully",
		ReviewID: req.ID,
	})
}
