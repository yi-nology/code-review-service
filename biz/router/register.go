package router

import (
	"github.com/cloudwego/hertz/pkg/app/server"
	rsRouter "github.com/yi-nology/code-review-service/biz/router/review_service"
	rsHandler "github.com/yi-nology/code-review-service/biz/handler/review_service"
)

func GeneratedRegister(r *server.Hertz, h *rsHandler.ReviewHandler, wh *rsHandler.WebhookHandler) {
	rsRouter.Register(r, h, wh)
}
