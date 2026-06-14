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

	// Rate limiting middleware
	h.Use(middleware.RateLimitMiddleware(100, 200))
	hlog.Infof("rate limiting middleware enabled (100 req/s, burst 200)")

	if cfg.Auth.Token != "" {
		h.Use(middleware.BearerTokenAuth(cfg.Auth.Token))
		hlog.Infof("auth middleware enabled")
	}

	h.Use(middleware.SecurityHeaders())
}

func registerCustomRoutes(h *server.Hertz, cfg *config.Config, db *gormDB.DB, handler *review_service.ReviewHandler, webhookHandler *review_service.WebhookHandler, statsHandler *review_service.StatsHandler, sessionHandler *review_service.SessionHandler, progressWSHandler *review_service.ProgressWSHandler, reviewMapHandler *review_service.ReviewMapHandler, publisherHandler *review_service.PublisherHandler, projectAgentHandler *review_service.ProjectAgentHandler) {
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

		// Session routes
		reviews := v1.Group("/reviews")
		{
			reviews.GET("/:id/sessions", sessionHandler.GetSessions)
			reviews.POST("/:id/sessions", sessionHandler.CreateSession)
			reviews.GET("/:id/sessions/active", sessionHandler.GetActiveSession)
			reviews.GET("/:id/sessions/:sessionId", sessionHandler.GetSession)
			reviews.PUT("/:id/sessions/:sessionId/phase", sessionHandler.UpdateSessionPhase)
			reviews.PUT("/:id/sessions/:sessionId/status", sessionHandler.UpdateSessionStatus)
			reviews.PUT("/:id/sessions/:sessionId/agent-results", sessionHandler.UpdateSessionAgentResults)
			reviews.PUT("/:id/sessions/:sessionId/discussion", sessionHandler.UpdateSessionDiscussion)
			reviews.PUT("/:id/sessions/:sessionId/final-report", sessionHandler.UpdateSessionFinalReport)
			reviews.POST("/:id/sessions/:sessionId/complete", sessionHandler.CompleteSession)
			reviews.POST("/:id/sessions/:sessionId/fail", sessionHandler.FailSession)
			reviews.POST("/:id/sessions/:sessionId/cancel", sessionHandler.CancelSession)

			// Progress WebSocket route
			reviews.GET("/:id/progress", progressWSHandler.HandleProgressWS)
			reviews.GET("/:id/progress/status", progressWSHandler.GetProgress)

			// Review Maps routes
			reviews.POST("/:id/maps", reviewMapHandler.GenerateMap)
			reviews.GET("/:id/maps", reviewMapHandler.GetMaps)
			reviews.GET("/:id/maps/:mapId", reviewMapHandler.GetMap)
			reviews.POST("/:id/maps/preview", reviewMapHandler.PreviewMap)

			// Session Map route
			reviews.GET("/:id/sessions/:sessionId/map", reviewMapHandler.GetSessionMap)

			// Publisher routes
			reviews.POST("/:id/publish", publisherHandler.PublishReview)
			reviews.GET("/:id/preview", publisherHandler.PreviewComment)
			reviews.PUT("/:id/publish", publisherHandler.UpdatePublishedComment)
		}

		// Active reviews progress
		v1.GET("/progress/active", progressWSHandler.GetActiveReviews)

		// Sessions list
		v1.GET("/sessions", sessionHandler.ListSessions)

		// Project Agent configuration routes
		settings := v1.Group("/settings")
		{
			settings.GET("/projects/:provider/:repo/agents", projectAgentHandler.GetProjectAgents)
			settings.PUT("/projects/:provider/:repo/agents", projectAgentHandler.UpdateProjectAgent)
			settings.POST("/projects/:provider/:repo/agents/batch", projectAgentHandler.BatchUpdateProjectAgents)
			settings.DELETE("/projects/:provider/:repo/agents/:agentId", projectAgentHandler.DeleteProjectAgent)
		}
	}
}
