package review_service

import (
	"context"

	"github.com/cloudwego/hertz/pkg/app"
)

func PublishReview(ctx context.Context, c *app.RequestContext) {
	if publisherHandlerInst != nil {
		publisherHandlerInst.PublishReview(ctx, c)
	}
}

func PreviewComment(ctx context.Context, c *app.RequestContext) {
	if publisherHandlerInst != nil {
		publisherHandlerInst.PreviewComment(ctx, c)
	}
}

func UpdatePublishedComment(ctx context.Context, c *app.RequestContext) {
	if publisherHandlerInst != nil {
		publisherHandlerInst.UpdatePublishedComment(ctx, c)
	}
}
