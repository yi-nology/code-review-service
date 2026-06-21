package middleware

import (
	"context"
	"strings"

	mw "github.com/oekit/middleware"
	"github.com/cloudwego/hertz/pkg/app"
)

// DefaultSkipPaths 是不需要认证和日志记录的默认路径列表。
var DefaultSkipPaths = []string{
	"/ping", "/healthz", "/health", "/ready", "/readyz",
	"/metrics", "/buildVersion",
}

// BearerTokenAuth 返回 Bearer Token 认证中间件。
// 委托给 ekit 的 mw.BearerTokenAuth。
func BearerTokenAuth(token string) app.HandlerFunc {
	return mw.BearerTokenAuth(mw.BearerTokenConfig{
		Token:     token,
		SkipPaths: DefaultSkipPaths,
		SkipFunc: func(path string) bool {
			return strings.HasPrefix(path, "/swagger") ||
				strings.HasPrefix(path, "/api/v1/webhooks")
		},
	})
}

// SecurityHeaders 返回安全响应头中间件。
// 包含适配 Element Plus 和 WebSocket 的 CSP 策略。
func SecurityHeaders() app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		c.Header("Content-Security-Policy",
			"default-src 'self'; "+
				"script-src 'self'; "+
				"style-src 'self' 'unsafe-inline'; "+
				"img-src 'self' data: blob:; "+
				"connect-src 'self' ws: wss:; "+
				"font-src 'self' data:; "+
				"frame-ancestors 'none'")
		c.Header("Referrer-Policy", "no-referrer")
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Next(ctx)
	}
}
