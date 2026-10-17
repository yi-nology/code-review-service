package middleware

import (
	"context"
	"strings"

	"github.com/cloudwego/hertz/pkg/app"
	mw "github.com/oekit/middleware/middleware"
)

// DefaultSkipPaths 是不需要认证和日志记录的默认路径列表。
var DefaultSkipPaths = []string{
	"/ping", "/healthz", "/health", "/ready", "/readyz",
	"/metrics", "/buildVersion",
}

// BearerTokenAuth 返回 Bearer Token 认证中间件。
// 委托给 ekit 的 mw.BearerTokenAuth。
func BearerTokenAuth(token string) app.HandlerFunc {
	return mw.BearerTokenAuth(
		mw.WithBearerToken(token),
		mw.WithAuthSkipPaths(DefaultSkipPaths...),
		mw.WithAuthSkipFunc(func(path string) bool {
			return strings.HasPrefix(path, "/swagger") ||
				strings.HasPrefix(path, "/openapi.yaml") ||
				strings.HasPrefix(path, "/api/v1/webhooks") ||
				strings.HasPrefix(path, "/api/v1/settings/ocr/viewer/proxy") ||
				// WebSocket upgrade 无法携带 Authorization header，progress WS 有 sameOrigin 校验防 CSRF。
				strings.HasSuffix(path, "/progress")
		}),
	)
}

// SecurityHeaders 返回安全响应头中间件。
// 包含适配 Element Plus 和 WebSocket 的 CSP 策略。
// Swagger 文档路径使用宽松策略以支持内联脚本、Google Fonts 及 iframe 嵌入。
func SecurityHeaders() app.HandlerFunc {
	appCSP := "default-src 'self'; " +
		"script-src 'self'; " +
		"style-src 'self' 'unsafe-inline'; " +
		"img-src 'self' data: blob:; " +
		"connect-src 'self' ws: wss:; " +
		"font-src 'self' data:; " +
		"frame-ancestors 'none'"

	swaggerCSP := "default-src 'self'; " +
		"script-src 'self' 'unsafe-inline'; " +
		"style-src 'self' 'unsafe-inline' https://fonts.googleapis.com; " +
		"img-src 'self' data: blob:; " +
		"connect-src 'self' ws: wss:; " +
		"font-src 'self' data: https://fonts.gstatic.com; " +
		"frame-ancestors 'self'"

	// Viewer 代理需要同源 iframe 嵌入；代理处理器会用 token 查询参数自行鉴权。
	viewerProxyCSP := "default-src 'self'; " +
		"script-src 'self' 'unsafe-inline'; " +
		"style-src 'self' 'unsafe-inline'; " +
		"img-src 'self' data: blob:; " +
		"connect-src 'self' ws: wss: http: https:; " +
		"font-src 'self' data:; " +
		"frame-ancestors 'self'"

	return func(ctx context.Context, c *app.RequestContext) {
		path := string(c.Request.URI().Path())
		switch {
		case strings.HasPrefix(path, "/swagger") || strings.HasPrefix(path, "/openapi.yaml"):
			c.Header("Content-Security-Policy", swaggerCSP)
			c.Header("X-Frame-Options", "SAMEORIGIN")
		case strings.HasPrefix(path, "/api/v1/settings/ocr/viewer/proxy"):
			c.Header("Content-Security-Policy", viewerProxyCSP)
			c.Header("X-Frame-Options", "SAMEORIGIN")
		default:
			c.Header("Content-Security-Policy", appCSP)
			c.Header("X-Frame-Options", "DENY")
		}
		c.Header("Referrer-Policy", "no-referrer")
		c.Header("X-Content-Type-Options", "nosniff")
		c.Next(ctx)
	}
}
