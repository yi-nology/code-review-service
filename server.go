package main

import (
	"fmt"
	"time"

	"github.com/oekit/hertz/health"
	emetrics "github.com/oekit/metrics"
	mw "github.com/oekit/middleware"
	"github.com/yi-nology/code-review-service/biz/router"
	"github.com/yi-nology/code-review-service/internal/config"
	"github.com/yi-nology/code-review-service/internal/middleware"
	"github.com/yi-nology/code-review-service/swagger"

	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/common/hlog"
	gormDB "gorm.io/gorm"
)

func createServer(cfg *config.Config) *server.Hertz {
	return server.Default(
		server.WithHostPorts(fmt.Sprintf(":%d", cfg.Port)),
		server.WithExitWaitTime(15*time.Second),
	)
}

func registerGlobalMiddleware(h *server.Hertz, cfg *config.Config) {
	stack := mw.DefaultMiddleware(mw.StackConfig{
		ServiceName: "review-service",
		SkipPaths:   []string{"/metrics", "/ping", "/health", "/buildVersion", "/swagger"},
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

func registerRoutes(h *server.Hertz, db *gormDB.DB) {
	hlog.Infof("[routes] registering routes...")

	healthOpts := []health.Option{}
	if sqlDB, err := db.DB(); err == nil {
		healthOpts = append(healthOpts, health.WithCheckers(health.DBChecker(sqlDB)))
	}
	health.RegisterBasicRoutes(h, healthOpts...)

	swagger.RegisterRoutes(h)

	router.GeneratedRegister(h)
}
