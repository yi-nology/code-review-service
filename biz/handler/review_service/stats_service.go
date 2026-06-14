package review_service

import (
	"context"

	"github.com/cloudwego/hertz/pkg/app"
)

func ListContributors(ctx context.Context, c *app.RequestContext) {
	if statsHandlerInst != nil {
		statsHandlerInst.ListContributors(ctx, c)
	}
}

func GetContributor(ctx context.Context, c *app.RequestContext) {
	if statsHandlerInst != nil {
		statsHandlerInst.GetContributor(ctx, c)
	}
}

func GetContributorTrend(ctx context.Context, c *app.RequestContext) {
	if statsHandlerInst != nil {
		statsHandlerInst.GetContributorTrend(ctx, c)
	}
}

func GetContributorCategories(ctx context.Context, c *app.RequestContext) {
	if statsHandlerInst != nil {
		statsHandlerInst.GetContributorCategories(ctx, c)
	}
}
