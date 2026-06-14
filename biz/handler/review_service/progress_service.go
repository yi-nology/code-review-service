package review_service

import (
	"context"

	"github.com/cloudwego/hertz/pkg/app"
)

func HandleProgressWS(ctx context.Context, c *app.RequestContext) {
	if progressWSHandlerInst != nil {
		progressWSHandlerInst.HandleProgressWS(ctx, c)
	}
}

func GetProgress(ctx context.Context, c *app.RequestContext) {
	if progressWSHandlerInst != nil {
		progressWSHandlerInst.GetProgress(ctx, c)
	}
}

func GetActiveReviews(ctx context.Context, c *app.RequestContext) {
	if progressWSHandlerInst != nil {
		progressWSHandlerInst.GetActiveReviews(ctx, c)
	}
}
