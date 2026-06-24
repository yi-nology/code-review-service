package modules

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/yi-nology/code-review-service/biz/router"
	"github.com/yi-nology/code-review-service/internal/config"
	"github.com/yi-nology/code-review-service/internal/middleware"
	"github.com/yi-nology/code-review-service/internal/repository"
	"github.com/yi-nology/code-review-service/internal/service/llm"
	"github.com/yi-nology/code-review-service/internal/service/ocr"
	"github.com/yi-nology/code-review-service/swagger"

	"github.com/oekit/hertz/health"
	mw "github.com/oekit/middleware"
	emetrics "github.com/oekit/metrics"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/oekit/logx"
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
	logx.Infof("rate limiting middleware enabled (100 req/s, burst 200)")

	if cfg.Auth.Token != "" {
		h.Use(middleware.BearerTokenAuth(cfg.Auth.Token))
		logx.Infof("auth middleware enabled")
	}

	h.Use(middleware.SecurityHeaders())
}

func RegisterRoutes(h *server.Hertz, db *gorm.DB, llmMetrics *llm.Metrics, repos *repository.RepoContainer, resolver *ocr.ConfigResolver) {
	logx.Infof("[routes] registering routes...")

	healthOpts := []health.Option{}
	if sqlDB, err := db.DB(); err == nil {
		healthOpts = append(healthOpts, health.WithCheckers(health.DBChecker(sqlDB)))
	}
	healthOpts = append(healthOpts,
		health.WithCheckers(health.NamedChecker("ocr-binary", ocrBinaryCheck(resolver))),
		health.WithCheckers(health.NamedChecker("llm-endpoint", llmEndpointCheck(repos, resolver))),
	)
	health.RegisterBasicRoutes(h, healthOpts...)

	// /healthz /readyz：k8s 探针常用路径（auth skip 白名单已含），复用就绪检查。
	// 之前这些路径未注册导致 k8s probe 404。
	probeHandler := health.NewHealthHandler(
		health.NamedChecker("db", dbCheck(db)),
		health.NamedChecker("ocr-binary", ocrBinaryCheck(resolver)),
		health.NamedChecker("llm-endpoint", llmEndpointCheck(repos, resolver)),
	)
	h.GET("/healthz", probeHandler)
	h.GET("/readyz", probeHandler)

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
