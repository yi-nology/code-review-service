package middleware

import (
	"context"
	"crypto/subtle"
	"net/http"
	"strings"

	"github.com/oekit/pkg/slicex"
	"github.com/cloudwego/hertz/pkg/app"
)

func BearerTokenAuth(token string) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		if token == "" {
			c.Next(ctx)
			return
		}

		path := string(c.Path())
		if isHealthOrMetricsPath(path) || strings.HasPrefix(path, "/swagger") || strings.HasPrefix(path, "/api/v1/webhooks") {
			c.Next(ctx)
			return
		}

		header := string(c.GetHeader("Authorization"))
		t, ok := strings.CutPrefix(header, "Bearer ")
		if !ok || subtle.ConstantTimeCompare([]byte(t), []byte(token)) != 1 {
			c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			c.Abort()
			return
		}

		c.Next(ctx)
	}
}

func isHealthOrMetricsPath(path string) bool {
	return slicex.Contains([]string{"/ping", "/healthz", "/metrics", "/health", "/ready"}, path)
}
