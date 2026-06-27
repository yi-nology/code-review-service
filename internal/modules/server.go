package modules

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	reviewHandler "github.com/yi-nology/code-review-service/biz/handler/review_service"
	settingsHandler "github.com/yi-nology/code-review-service/biz/handler/settings_service"
	statsHandler "github.com/yi-nology/code-review-service/biz/handler/stats_service"
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

	"github.com/oekit/logx"
	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
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

func RegisterRoutes(h *server.Hertz, db *gorm.DB, llmMetrics *llm.Metrics, repos *repository.RepoContainer, resolver *ocr.ConfigResolver, viewerMgr *ocr.ViewerManager) {
	logx.Infof("[routes] registering routes...")

	healthOpts := []health.Option{}
	if sqlDB, err := db.DB(); err == nil {
		healthOpts = append(healthOpts, health.WithCheckers(health.DBChecker(sqlDB)))
	}
	healthOpts = append(healthOpts,
		health.WithCheckers(health.NamedChecker("ocr-binary", ocrBinaryCheck(repos, resolver))),
		health.WithCheckers(health.NamedChecker("llm-endpoint", llmEndpointCheck(repos, resolver))),
	)
	health.RegisterBasicRoutes(h, healthOpts...)

	// /healthz /readyz：k8s 探针常用路径（auth skip 白名单已含），复用就绪检查。
	// 之前这些路径未注册导致 k8s probe 404。
	probeHandler := health.NewHealthHandler(
		health.NamedChecker("db", dbCheck(db)),
		health.NamedChecker("ocr-binary", ocrBinaryCheck(repos, resolver)),
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
	registerSuggestionPatchRoutes(h)
	registerOCRAssetRoutes(h, viewerMgr)
}

func registerSuggestionPatchRoutes(h *server.Hertz) {
	api := h.Group("/api/v1/reviews")
	api.GET("/:id/suggestion-patch", reviewHandler.PreviewSuggestionPatch)
	api.GET("/:id/suggestion-patch/check", reviewHandler.CheckSuggestionPatch)
	api.GET("/:id/suggestion-patch.diff", reviewHandler.DownloadSuggestionPatch)
}

func registerOCRAssetRoutes(h *server.Hertz, viewerMgr *ocr.ViewerManager) {
	api := h.Group("/api/v1/settings")
	api.GET("/rules/packages", settingsHandler.ListRulePackages)
	api.POST("/rules/packages", settingsHandler.UpsertRulePackage)
	api.PUT("/rules/packages/:id", settingsHandler.UpsertRulePackage)
	api.DELETE("/rules/packages/:id", settingsHandler.DeleteRulePackage)

	api.GET("/ocr/tool-packages", settingsHandler.ListToolPackages)
	api.POST("/ocr/tool-packages", settingsHandler.UpsertToolPackage)
	api.PUT("/ocr/tool-packages/:id", settingsHandler.UpsertToolPackage)
	api.DELETE("/ocr/tool-packages/:id", settingsHandler.DeleteToolPackage)
	api.POST("/ocr/tool-packages/validate", settingsHandler.ValidateToolPackage)
	api.GET("/ocr/viewer/proxy/*path", viewerProxyHandler(viewerMgr))

	statsAPI := h.Group("/api/v1/stats")
	statsAPI.GET("/projects/ocr", statsHandler.GetProjectOCRMetrics)
}

func viewerProxyHandler(viewerMgr *ocr.ViewerManager) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		if viewerMgr == nil {
			c.SetStatusCode(http.StatusServiceUnavailable)
			return
		}
		status := viewerMgr.Status()
		if !status.Running || status.Addr == "" {
			c.SetStatusCode(http.StatusServiceUnavailable)
			_, _ = c.WriteString("OCR viewer is not running")
			return
		}
		path := c.Param("path")
		if path == "" || path == "/" {
			path = "/"
		}
		if !strings.HasPrefix(path, "/") {
			path = "/" + path
		}
		target := "http://" + status.Addr + path
		if query := string(c.Request.URI().QueryString()); query != "" {
			target += "?" + query
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
		if err != nil {
			c.SetStatusCode(http.StatusBadRequest)
			return
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			c.SetStatusCode(http.StatusBadGateway)
			_, _ = c.WriteString(err.Error())
			return
		}
		defer resp.Body.Close()
		for key, values := range resp.Header {
			if len(values) > 0 {
				c.Response.Header.Set(key, values[0])
			}
		}
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			c.SetStatusCode(http.StatusBadGateway)
			return
		}
		c.SetStatusCode(resp.StatusCode)
		_, _ = c.Write(body)
	}
}
