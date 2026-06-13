package router

import (
	"github.com/cloudwego/hertz/pkg/app/server"
	rsRouter "review-service/biz/router/review_service"
	rsHandler "review-service/biz/handler/review_service"
)

func GeneratedRegister(r *server.Hertz, h *rsHandler.ReviewHandler, wh *rsHandler.WebhookHandler) {
	rsRouter.Register(r, h, wh)
}
