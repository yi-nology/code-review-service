package review_service

import (
	"context"

	"github.com/cloudwego/hertz/pkg/app"
)

func CreateReview(ctx context.Context, c *app.RequestContext) {
	if reviewHandlerInst != nil {
		reviewHandlerInst.CreateReview(ctx, c)
	}
}

func GetReview(ctx context.Context, c *app.RequestContext) {
	if reviewHandlerInst != nil {
		reviewHandlerInst.GetReview(ctx, c)
	}
}

func ListReviews(ctx context.Context, c *app.RequestContext) {
	if reviewHandlerInst != nil {
		reviewHandlerInst.ListReviews(ctx, c)
	}
}

func PreviewReview(ctx context.Context, c *app.RequestContext) {
	if reviewHandlerInst != nil {
		reviewHandlerInst.PreviewReview(ctx, c)
	}
}

func RerunReview(ctx context.Context, c *app.RequestContext) {
	if reviewHandlerInst != nil {
		reviewHandlerInst.RerunReview(ctx, c)
	}
}

func CancelReview(ctx context.Context, c *app.RequestContext) {
	if reviewHandlerInst != nil {
		reviewHandlerInst.CancelReview(ctx, c)
	}
}

func HandleWebhook(ctx context.Context, c *app.RequestContext) {
	if webhookHandlerInst != nil {
		webhookHandlerInst.HandleWebhook(ctx, c)
	}
}

func ListProviderRepos(ctx context.Context, c *app.RequestContext) {
	if reviewHandlerInst != nil {
		reviewHandlerInst.ListProviderRepos(ctx, c)
	}
}

func ListProviderBranches(ctx context.Context, c *app.RequestContext) {
	if reviewHandlerInst != nil {
		reviewHandlerInst.ListProviderBranches(ctx, c)
	}
}

func GetBranchSHA(ctx context.Context, c *app.RequestContext) {
	if reviewHandlerInst != nil {
		reviewHandlerInst.GetBranchSHA(ctx, c)
	}
}

func ListProviderPulls(ctx context.Context, c *app.RequestContext) {
	if reviewHandlerInst != nil {
		reviewHandlerInst.ListProviderPulls(ctx, c)
	}
}
