package main

import (
	"fmt"
	"time"

	"github.com/oekit/hertz/health"
	emetrics "github.com/oekit/metrics"
	mw "github.com/oekit/middleware"
	"github.com/yi-nology/code-review-service/biz/handler/review_service"
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

	if cfg.Auth.Token != "" {
		h.Use(middleware.BearerTokenAuth(cfg.Auth.Token))
		hlog.Infof("auth middleware enabled")
	}

	h.Use(middleware.SecurityHeaders())
}

func registerCustomRoutes(h *server.Hertz, cfg *config.Config, db *gormDB.DB, handler *review_service.ReviewHandler, webhookHandler *review_service.WebhookHandler, statsHandler *review_service.StatsHandler) {
	hlog.Infof("[routes] registering custom routes...")

	healthOpts := []health.Option{}
	if sqlDB, err := db.DB(); err == nil {
		healthOpts = append(healthOpts, health.WithCheckers(health.DBChecker(sqlDB)))
	}
	health.RegisterBasicRoutes(h, healthOpts...)

	swagger.RegisterRoutes(h)

	router.GeneratedRegister(h, handler, webhookHandler)

	v1 := h.Group("/api/v1")
	{
		stats := v1.Group("/stats")
		stats.GET("/contributors", statsHandler.ListContributors)
		stats.GET("/contributors/:author", statsHandler.GetContributor)
		stats.GET("/contributors/:author/trend", statsHandler.GetContributorTrend)
		stats.GET("/contributors/:author/categories", statsHandler.GetContributorCategories)
	}
}
