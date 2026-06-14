package review_service

import (
	"context"

	"github.com/cloudwego/hertz/pkg/app"
)

func ListSessions(ctx context.Context, c *app.RequestContext) {
	if sessionHandlerInst != nil {
		sessionHandlerInst.ListSessions(ctx, c)
	}
}

func GetSessions(ctx context.Context, c *app.RequestContext) {
	if sessionHandlerInst != nil {
		sessionHandlerInst.GetSessions(ctx, c)
	}
}

func CreateSession(ctx context.Context, c *app.RequestContext) {
	if sessionHandlerInst != nil {
		sessionHandlerInst.CreateSession(ctx, c)
	}
}

func GetActiveSession(ctx context.Context, c *app.RequestContext) {
	if sessionHandlerInst != nil {
		sessionHandlerInst.GetActiveSession(ctx, c)
	}
}

func GetSession(ctx context.Context, c *app.RequestContext) {
	if sessionHandlerInst != nil {
		sessionHandlerInst.GetSession(ctx, c)
	}
}

func UpdateSessionPhase(ctx context.Context, c *app.RequestContext) {
	if sessionHandlerInst != nil {
		sessionHandlerInst.UpdateSessionPhase(ctx, c)
	}
}

func UpdateSessionStatus(ctx context.Context, c *app.RequestContext) {
	if sessionHandlerInst != nil {
		sessionHandlerInst.UpdateSessionStatus(ctx, c)
	}
}

func UpdateSessionAgentResults(ctx context.Context, c *app.RequestContext) {
	if sessionHandlerInst != nil {
		sessionHandlerInst.UpdateSessionAgentResults(ctx, c)
	}
}

func UpdateSessionDiscussion(ctx context.Context, c *app.RequestContext) {
	if sessionHandlerInst != nil {
		sessionHandlerInst.UpdateSessionDiscussion(ctx, c)
	}
}

func UpdateSessionFinalReport(ctx context.Context, c *app.RequestContext) {
	if sessionHandlerInst != nil {
		sessionHandlerInst.UpdateSessionFinalReport(ctx, c)
	}
}

func CompleteSession(ctx context.Context, c *app.RequestContext) {
	if sessionHandlerInst != nil {
		sessionHandlerInst.CompleteSession(ctx, c)
	}
}

func FailSession(ctx context.Context, c *app.RequestContext) {
	if sessionHandlerInst != nil {
		sessionHandlerInst.FailSession(ctx, c)
	}
}

func CancelSession(ctx context.Context, c *app.RequestContext) {
	if sessionHandlerInst != nil {
		sessionHandlerInst.CancelSession(ctx, c)
	}
}
