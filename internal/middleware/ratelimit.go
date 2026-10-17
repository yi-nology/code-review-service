package middleware

import (
	mw "github.com/oekit/middleware/middleware"
	"github.com/cloudwego/hertz/pkg/app"
	"golang.org/x/time/rate"
)

// RateLimitMiddleware 返回限流中间件。
// 委托给 ekit 的 mw.RateLimitMiddleware：默认按 ClientIP 分桶（KeyedLimiter，每 key
// 独立令牌桶 + TTL 清理），非全局共享预算；需要全局预算时显式传 WithRateLimitKeyFunc。
func RateLimitMiddleware(rps int, burst int) app.HandlerFunc {
	return mw.RateLimitMiddleware(mw.WithRateLimit(rate.Limit(rps), burst))
}
