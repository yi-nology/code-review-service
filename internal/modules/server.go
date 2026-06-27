package modules

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	reviewHandler "github.com/yi-nology/code-review-service/biz/handler/review_service"
	settingsHandler "github.com/yi-nology/code-review-service/biz/handler/settings_service"
	"github.com/yi-nology/code-review-service/biz/router"
	"github.com/yi-nology/code-review-service/internal/config"
	"github.com/yi-nology/code-review-service/internal/middleware"
	"github.com/yi-nology/code-review-service/internal/readiness"
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

	rps, burst := cfg.GetRateLimit()
	h.Use(middleware.RateLimitMiddleware(rps, burst))
	logx.Infof("rate limiting middleware enabled (%d req/s, burst %d)", rps, burst)

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
		health.WithCheckers(health.NamedChecker("ocr-binary", readiness.OCRBinaryCheck(repos, resolver))),
		health.WithCheckers(health.NamedChecker("llm-endpoint", readiness.LLMEndpointCheck(repos, resolver))),
	)
	health.RegisterBasicRoutes(h, healthOpts...)

	// /healthz /readyz：k8s 探针常用路径（auth skip 白名单已含），复用就绪检查。
	// 之前这些路径未注册导致 k8s probe 404。
	probeHandler := health.NewHealthHandler(
		health.NamedChecker("db", readiness.DBCheck(db)),
		health.NamedChecker("ocr-binary", readiness.OCRBinaryCheck(repos, resolver)),
		health.NamedChecker("llm-endpoint", readiness.LLMEndpointCheck(repos, resolver)),
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
	registerReviewExtraRoutes(h)
	registerSettingsExtraRoutes(h)
	registerViewerProxyRoute(h, viewerMgr)
}

func registerReviewExtraRoutes(h *server.Hertz) {
	h.POST("/api/v1/reviews/:id/suggestion-patch/apply", reviewHandler.ApplySuggestionPatch)
	h.POST("/api/v1/reviews/:id/model-compare", reviewHandler.CompareReviewModels)
}

func registerSettingsExtraRoutes(h *server.Hertz) {
	h.GET("/api/v1/settings/rules/packages/:id/history", settingsHandler.ListRulePackageVersions)
	h.POST("/api/v1/settings/rules/packages/:id/history/:version/restore", settingsHandler.RestoreRulePackageVersion)
	h.GET("/api/v1/settings/ocr/tool-packages/:id/history", settingsHandler.ListToolPackageVersions)
	h.POST("/api/v1/settings/ocr/tool-packages/:id/history/:version/restore", settingsHandler.RestoreToolPackageVersion)
}

func registerViewerProxyRoute(h *server.Hertz, viewerMgr *ocr.ViewerManager) {
	h.GET("/api/v1/settings/ocr/viewer/proxy/*path", viewerProxyHandler(viewerMgr))
}

func viewerProxyHandler(viewerMgr *ocr.ViewerManager) app.HandlerFunc {
	proxyClient := &http.Client{Timeout: 30 * time.Second}
	const maxProxyBody = 10 << 20 // 10 MB

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
		resp, err := proxyClient.Do(req)
		if err != nil {
			c.SetStatusCode(http.StatusBadGateway)
			_, _ = c.WriteString("upstream request failed")
			return
		}
		defer resp.Body.Close()
		for key, values := range resp.Header {
			if len(values) > 0 {
				c.Response.Header.Set(key, values[0])
			}
		}
		body, err := io.ReadAll(io.LimitReader(resp.Body, maxProxyBody))
		if err != nil {
			c.SetStatusCode(http.StatusBadGateway)
			return
		}
		c.SetStatusCode(resp.StatusCode)
		_, _ = c.Write(body)
	}
}
