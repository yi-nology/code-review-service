package modules

import (
	"github.com/yi-nology/code-review-service/biz/router"
	"github.com/yi-nology/code-review-service/internal/config"
	"github.com/yi-nology/code-review-service/internal/middleware"
	"github.com/yi-nology/code-review-service/swagger"

	"github.com/oekit/hertz/health"
	mw "github.com/oekit/middleware"
	emetrics "github.com/oekit/metrics"

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

func RegisterRoutes(h *server.Hertz, db *gorm.DB) {
	hlog.Infof("[routes] registering routes...")

	healthOpts := []health.Option{}
	if sqlDB, err := db.DB(); err == nil {
		healthOpts = append(healthOpts, health.WithCheckers(health.DBChecker(sqlDB)))
	}
	health.RegisterBasicRoutes(h, healthOpts...)

	swagger.RegisterRoutes(h)

	router.GeneratedRegister(h)
}
