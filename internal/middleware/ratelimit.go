package middleware

import (
	mw "github.com/oekit/middleware"
	"github.com/cloudwego/hertz/pkg/app"
	"golang.org/x/time/rate"
)

// RateLimitMiddleware 返回限流中间件。
// 委托给 ekit 的 mw.RateLimitMiddleware。
func RateLimitMiddleware(rps int, burst int) app.HandlerFunc {
	return mw.RateLimitMiddleware(mw.WithRateLimit(rate.Limit(rps), burst))
}
