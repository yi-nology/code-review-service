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
	req, ok := common.Bind[modelPublisher.PublishReviewReq](c)
	if !ok { return }

	common.Success(c, &modelPublisher.PublishReviewResp{
		Message:  "review published successfully",
		ReviewID: req.ID,
	})
}

func PreviewComment(ctx context.Context, c *app.RequestContext) {
	req, ok := common.Bind[modelPublisher.PreviewCommentReq](c)
	if !ok { return }

	common.Success(c, &modelPublisher.PreviewCommentResp{
		ReviewID: req.ID,
		Summary:  "",
		Findings: "",
		Total:    0,
	})
}

func UpdatePublishedComment(ctx context.Context, c *app.RequestContext) {
	req, ok := common.Bind[modelPublisher.UpdatePublishedCommentReq](c)
	if !ok { return }

	common.Success(c, &modelPublisher.UpdatePublishedCommentResp{
		Message:  "published comment updated successfully",
		ReviewID: req.ID,
	})
}
