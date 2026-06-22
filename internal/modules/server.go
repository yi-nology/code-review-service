package modules

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/yi-nology/code-review-service/biz/router"
	"github.com/yi-nology/code-review-service/internal/config"
	"github.com/yi-nology/code-review-service/internal/middleware"
	"github.com/yi-nology/code-review-service/internal/service/llm"
	"github.com/yi-nology/code-review-service/swagger"

	"github.com/oekit/hertz/health"
	mw "github.com/oekit/middleware"
	emetrics "github.com/oekit/metrics"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/common/hlog"
	"go.uber.org/fx"
	"gorm.io/gorm"
)

var ServerModule = fx.Options(
	fx.Invoke(RegisterGlobalMiddleware),
	fx.Invoke(RegisterRoutes),
)

func RegisterGlobalMiddleware(h *server.Hertz, cfg *config.Config) {
	h.Use(mw.RecoveryMiddleware())
	h.Use(mw.CORSMiddleware())

	skipPaths := middleware.DefaultSkipPaths
	stack := mw.DefaultMiddleware(mw.StackConfig{
		ServiceName: "review-service",
		SkipPaths:   skipPaths,
		Registry:    emetrics.Registry,
	})
	for _, m := range stack {
		h.Use(m)
	}

	h.Use(middleware.RateLimitMiddleware(100, 200))
	hlog.Infof("rate limiting middleware enabled (100 req/s, burst 200)")

	if cfg.Auth.Token != "" {
		h.Use(middleware.BearerTokenAuth(cfg.Auth.Token))
		hlog.Infof("auth middleware enabled")
	}

	h.Use(middleware.SecurityHeaders())
}

func RegisterRoutes(h *server.Hertz, db *gorm.DB, llmMetrics *llm.Metrics) {
	hlog.Infof("[routes] registering routes...")

	healthOpts := []health.Option{}
	if sqlDB, err := db.DB(); err == nil {
		healthOpts = append(healthOpts, health.WithCheckers(health.DBChecker(sqlDB)))
	}
	health.RegisterBasicRoutes(h, healthOpts...)

	// LLM 调用指标端点（阶段 6 可观测）
	h.GET("/debug/llm/metrics", func(ctx context.Context, c *app.RequestContext) {
		snapshot := llmMetrics.Snapshot()
		body, err := json.Marshal(snapshot)
		if err != nil {
			c.SetStatusCode(http.StatusInternalServerError)
			return
		}
		c.SetStatusCode(http.StatusOK)
		c.SetContentTypeBytes([]byte("application/json"))
		_, _ = c.Write(body)
	})

	swagger.RegisterRoutes(h)

	router.GeneratedRegister(h)
}
