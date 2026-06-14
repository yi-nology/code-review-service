package review_service

import (
	"context"

	"github.com/cloudwego/hertz/pkg/app"
)

func GetProjectAgents(ctx context.Context, c *app.RequestContext) {
	if projectAgentHandlerInst != nil {
		projectAgentHandlerInst.GetProjectAgents(ctx, c)
	}
}

func UpdateProjectAgent(ctx context.Context, c *app.RequestContext) {
	if projectAgentHandlerInst != nil {
		projectAgentHandlerInst.UpdateProjectAgent(ctx, c)
	}
}

func BatchUpdateProjectAgents(ctx context.Context, c *app.RequestContext) {
	if projectAgentHandlerInst != nil {
		projectAgentHandlerInst.BatchUpdateProjectAgents(ctx, c)
	}
}

func DeleteProjectAgent(ctx context.Context, c *app.RequestContext) {
	if projectAgentHandlerInst != nil {
		projectAgentHandlerInst.DeleteProjectAgent(ctx, c)
	}
}
