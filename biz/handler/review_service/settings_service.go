package review_service

import (
	"context"

	"github.com/cloudwego/hertz/pkg/app"
)

func GetOCRSettings(ctx context.Context, c *app.RequestContext) {
	if reviewHandlerInst != nil {
		reviewHandlerInst.GetOCRSettings(ctx, c)
	}
}

func UpdateOCRSettings(ctx context.Context, c *app.RequestContext) {
	if reviewHandlerInst != nil {
		reviewHandlerInst.UpdateOCRSettings(ctx, c)
	}
}

func TestLLM(ctx context.Context, c *app.RequestContext) {
	if reviewHandlerInst != nil {
		reviewHandlerInst.TestLLM(ctx, c)
	}
}

func ListProjectPolicies(ctx context.Context, c *app.RequestContext) {
	if reviewHandlerInst != nil {
		reviewHandlerInst.ListProjectPolicies(ctx, c)
	}
}

func PutProjectPolicy(ctx context.Context, c *app.RequestContext) {
	if reviewHandlerInst != nil {
		reviewHandlerInst.PutProjectPolicy(ctx, c)
	}
}

func DeleteProjectPolicy(ctx context.Context, c *app.RequestContext) {
	if reviewHandlerInst != nil {
		reviewHandlerInst.DeleteProjectPolicy(ctx, c)
	}
}

func ListPlatforms(ctx context.Context, c *app.RequestContext) {
	if reviewHandlerInst != nil {
		reviewHandlerInst.ListPlatforms(ctx, c)
	}
}

func PutPlatform(ctx context.Context, c *app.RequestContext) {
	if reviewHandlerInst != nil {
		reviewHandlerInst.PutPlatform(ctx, c)
	}
}

func DeletePlatform(ctx context.Context, c *app.RequestContext) {
	if reviewHandlerInst != nil {
		reviewHandlerInst.DeletePlatform(ctx, c)
	}
}

func TestPlatform(ctx context.Context, c *app.RequestContext) {
	if reviewHandlerInst != nil {
		reviewHandlerInst.TestPlatform(ctx, c)
	}
}

func GetGlobalRules(ctx context.Context, c *app.RequestContext) {
	if reviewHandlerInst != nil {
		reviewHandlerInst.GetGlobalRules(ctx, c)
	}
}

func UpdateGlobalRules(ctx context.Context, c *app.RequestContext) {
	if reviewHandlerInst != nil {
		reviewHandlerInst.UpdateGlobalRules(ctx, c)
	}
}

func ListReviewerAgents(ctx context.Context, c *app.RequestContext) {
	if reviewHandlerInst != nil {
		reviewHandlerInst.ListReviewerAgents(ctx, c)
	}
}

func CreateReviewerAgent(ctx context.Context, c *app.RequestContext) {
	if reviewHandlerInst != nil {
		reviewHandlerInst.CreateReviewerAgent(ctx, c)
	}
}

func UpdateReviewerAgent(ctx context.Context, c *app.RequestContext) {
	if reviewHandlerInst != nil {
		reviewHandlerInst.UpdateReviewerAgent(ctx, c)
	}
}

func DeleteReviewerAgent(ctx context.Context, c *app.RequestContext) {
	if reviewHandlerInst != nil {
		reviewHandlerInst.DeleteReviewerAgent(ctx, c)
	}
}
