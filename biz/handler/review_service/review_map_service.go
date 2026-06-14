package review_service

import (
	"context"

	"github.com/cloudwego/hertz/pkg/app"
)

func GenerateMap(ctx context.Context, c *app.RequestContext) {
	if reviewMapHandlerInst != nil {
		reviewMapHandlerInst.GenerateMap(ctx, c)
	}
}

func GetMaps(ctx context.Context, c *app.RequestContext) {
	if reviewMapHandlerInst != nil {
		reviewMapHandlerInst.GetMaps(ctx, c)
	}
}

func GetMap(ctx context.Context, c *app.RequestContext) {
	if reviewMapHandlerInst != nil {
		reviewMapHandlerInst.GetMap(ctx, c)
	}
}

func PreviewMap(ctx context.Context, c *app.RequestContext) {
	if reviewMapHandlerInst != nil {
		reviewMapHandlerInst.PreviewMap(ctx, c)
	}
}

func GetSessionMap(ctx context.Context, c *app.RequestContext) {
	if reviewMapHandlerInst != nil {
		reviewMapHandlerInst.GetSessionMap(ctx, c)
	}
}
