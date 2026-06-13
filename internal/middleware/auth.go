package middleware

import (
	"context"
	"crypto/subtle"
	"net/http"
	"strings"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/hlog"
)

type AuthConfig struct {
	APIKeys    []string
	SkipPaths  []string
	HeaderName string
	KeyPrefix  string
}

func AuthMiddleware(config AuthConfig) app.HandlerFunc {
	keySet := make(map[string]bool, len(config.APIKeys))
	for _, key := range config.APIKeys {
		if key != "" {
			keySet[key] = true
		}
	}

	skipPaths := make(map[string]bool, len(config.SkipPaths))
	for _, path := range config.SkipPaths {
		skipPaths[path] = true
	}

	headerName := config.HeaderName
	if headerName == "" {
		headerName = "Authorization"
	}
	keyPrefix := config.KeyPrefix
	if keyPrefix == "" {
		keyPrefix = "Bearer "
	}

	return func(ctx context.Context, c *app.RequestContext) {
		path := string(c.Path())

		if skipPaths[path] {
			c.Next(ctx)
			return
		}

		if isHealthOrMetricsPath(path) {
			c.Next(ctx)
			return
		}

		if strings.HasPrefix(path, "/swagger") {
			c.Next(ctx)
			return
		}

		authHeader := c.GetHeader(headerName)
		if len(authHeader) == 0 {
			hlog.Warnf("missing auth header for path: %s", path)
			c.JSON(http.StatusUnauthorized, map[string]interface{}{
				"code":    401,
				"message": "missing authorization header",
				"data":    nil,
			})
			c.Abort()
			return
		}

		token := string(authHeader)
		token = strings.TrimPrefix(token, keyPrefix)

		matched := false
		for k := range keySet {
			if subtle.ConstantTimeCompare([]byte(token), []byte(k)) == 1 {
				matched = true
				break
			}
		}
		if !matched {
			hlog.Warnf("invalid api key for path: %s", path)
			c.JSON(http.StatusUnauthorized, map[string]interface{}{
				"code":    401,
				"message": "invalid api key",
				"data":    nil,
			})
			c.Abort()
			return
		}

		c.Next(ctx)
	}
}

func BearerTokenAuth(token string) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		if token == "" {
			c.Next(ctx)
			return
		}

		path := string(c.Path())
		if isHealthOrMetricsPath(path) {
			c.Next(ctx)
			return
		}

		if strings.HasPrefix(path, "/swagger") {
			c.Next(ctx)
			return
		}

		if strings.HasPrefix(path, "/api/v1/webhooks") {
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
	healthPaths := []string{"/ping", "/healthz", "/metrics", "/health", "/ready"}
	for _, hp := range healthPaths {
		if path == hp {
			return true
		}
	}
	return false
}
