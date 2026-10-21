package modules

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	authHandler "github.com/yi-nology/code-review-service/biz/handler/auth_service"
	"github.com/yi-nology/code-review-service/biz/handler/common"
	costHandler "github.com/yi-nology/code-review-service/biz/handler/cost_service"
	diagnosticsHandler "github.com/yi-nology/code-review-service/biz/handler/diagnostics_service"
	feedbackHandler "github.com/yi-nology/code-review-service/biz/handler/feedback_service"
	notificationHandler "github.com/yi-nology/code-review-service/biz/handler/notification_service"
	reportHandler "github.com/yi-nology/code-review-service/biz/handler/report_service"
	reviewHandler "github.com/yi-nology/code-review-service/biz/handler/review_service"
	settingsHandler "github.com/yi-nology/code-review-service/biz/handler/settings_service"
	"github.com/yi-nology/code-review-service/biz/router"
	"github.com/yi-nology/code-review-service/internal/config"
	"github.com/yi-nology/code-review-service/internal/middleware"
	ocrcore "github.com/yi-nology/code-review-service/internal/ocr"
	"github.com/yi-nology/code-review-service/internal/readiness"
	"github.com/yi-nology/code-review-service/internal/repository"

	// blank import：触发业务指标包的 init()，将指标注册到 ekit Registry。
	_ "github.com/yi-nology/code-review-service/internal/observability/metrics"
	"github.com/yi-nology/code-review-service/swagger"

	"github.com/oekit/hertz/health"
	"github.com/oekit/hertz/metrics"
	emetrics "github.com/oekit/metrics"
	mw "github.com/oekit/middleware"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/hertz-contrib/cors"
	"github.com/oekit/logx"
	"go.uber.org/fx"
	"gorm.io/gorm"
)

var ServerModule = fx.Options(
	fx.Invoke(RegisterGlobalMiddleware),
	fx.Invoke(RegisterRoutes),
)

func RegisterGlobalMiddleware(h *server.Hertz, cfg *config.Config, repos *repository.RepoContainer) {
	h.Use(cors.Default())

	skipPaths := middleware.DefaultSkipPaths
	// oekit 零指标强制：Prometheus 显式挂栈首（DefaultMiddleware 栈内自带 Recovery，
	// 开头显式的 mw.RecoveryMiddleware() 为存量双挂，一并对齐移除由栈内承担）
	pm := mw.NewPrometheusMetrics(mw.PrometheusConfig{
		ServiceName: "review-service",
		SkipPaths:   skipPaths,
		Registry:    emetrics.Registry,
	})
	h.Use(pm.Middleware())
	for _, m := range mw.DefaultMiddleware(mw.StackConfig{SkipPaths: skipPaths}) {
		h.Use(m)
	}

	rps, burst := cfg.GetRateLimit()
	h.Use(middleware.RateLimitMiddleware(rps, burst))
	logx.Infof("rate limiting middleware enabled (%d req/s, burst %d)", rps, burst)

	// 多 Token 鉴权：优先查 DB api_tokens 表（具名 token + scope + 审计），
	// 未命中时 fallback 到配置的单 token（向后兼容）。
	if repos != nil && repos.Auth != nil {
		h.Use(middleware.MultiTokenAuth(middleware.NewRepoTokenSource(repos.Auth), cfg.Auth.Token))
		logx.Infof("multi-token auth middleware enabled (DB tokens + fallback single token)")
	} else if cfg.Auth.Token != "" {
		h.Use(middleware.BearerTokenAuth(cfg.Auth.Token))
		logx.Infof("auth middleware enabled (single token)")
	}

	// 审计中间件：记录写操作的「谁/何时/做了什么」。
	if repos != nil && repos.Auth != nil {
		h.Use(middleware.AuditMiddleware(repos.Auth))
	}

	h.Use(middleware.SecurityHeaders())
}

// RegisterRoutes 收口为 5 个参数：h + db + llmMetrics + *ocrcore.Registry + cfg。
// 之前 7 个参数；repos/resolver/viewerMgr 全部从 *ocrcore.Registry 拿。
func RegisterRoutes(h *server.Hertz, db *gorm.DB, llmMetrics *ocrcore.Metrics, ocrReg *ocrcore.Registry, cfg *config.Config) {
	logx.Infof("[routes] registering routes...")

	repos := ocrReg.Repos()
	resolver := ocrReg.Resolver()

	healthOpts := []health.HealthOption{}
	if sqlDB, err := db.DB(); err == nil {
		healthOpts = append(healthOpts, health.WithCheckers(health.DBChecker(sqlDB)))
	}
	healthOpts = append(healthOpts,
		health.WithCheckers(health.NamedChecker("ocr-binary", readiness.OCRBinaryCheck(repos, resolver))),
		health.WithCheckers(health.NamedChecker("llm-endpoint", readiness.LLMEndpointCheck(repos, resolver))),
	)
	health.RegisterHealthRoutes(h, healthOpts...)

	// /healthz /readyz：k8s 探针常用路径（auth skip 白名单已含），复用就绪检查。
	// 之前这些路径未注册导致 k8s probe 404。
	probeHandler := health.NewHealthHandler(
		health.WithCheckers(
			health.NamedChecker("db", readiness.DBCheck(db)),
			health.NamedChecker("ocr-binary", readiness.OCRBinaryCheck(repos, resolver)),
			health.NamedChecker("llm-endpoint", readiness.LLMEndpointCheck(repos, resolver)),
		),
	)
	h.GET("/healthz", probeHandler)
	h.GET("/readyz", probeHandler)

	// /metrics：Prometheus 抓取端点（auth skip 白名单已含）。
	// 暴露 ekit Registry 上注册的全部指标（build_info、HTTP、Go runtime、业务指标）。
	h.GET("/metrics", metrics.MetricsHandler())

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
	registerReviewExtraRoutes(h, repos)
	registerDelegateRoutes(h)
	registerSettingsExtraRoutes(h)
	registerDiagnosticsRoutes(h)
	registerFeedbackRoutes(h)
	registerNotificationRoutes(h)
	registerAuthRoutes(h)
	registerCostRoutes(h)
	registerReportRoutes(h)
	registerViewerProxyRoute(h, ocrReg.Viewer().(*ocrcore.Viewer), cfg.Auth.Token)
}

func registerReportRoutes(h *server.Hertz) {
	h.GET("/api/v1/reports/weekly", reportHandler.GenerateWeeklyReport)
}

func registerCostRoutes(h *server.Hertz) {
	h.GET("/api/v1/settings/cost/pricing", costHandler.ListPricing)
	h.POST("/api/v1/settings/cost/pricing", costHandler.UpsertPricing)
	h.DELETE("/api/v1/settings/cost/pricing/:id", costHandler.DeletePricing)
}

func registerNotificationRoutes(h *server.Hertz) {
	h.GET("/api/v1/notifications", notificationHandler.List)
	h.GET("/api/v1/notifications/unread-count", notificationHandler.UnreadCount)
	h.POST("/api/v1/notifications/:id/read", notificationHandler.MarkRead)
	h.POST("/api/v1/notifications/read-all", notificationHandler.MarkAllRead)
}

func registerAuthRoutes(h *server.Hertz) {
	h.POST("/api/v1/settings/tokens", authHandler.CreateToken)
	h.GET("/api/v1/settings/tokens", authHandler.ListTokens)
	h.DELETE("/api/v1/settings/tokens/:id", authHandler.DeleteToken)
	h.GET("/api/v1/settings/audit-logs", authHandler.ListAudit)
}

func registerReviewExtraRoutes(h *server.Hertz, repos *repository.RepoContainer) {
	h.POST("/api/v1/reviews/:id/suggestion-patch/apply", reviewHandler.ApplySuggestionPatch)
	// 死信重排队：把因重试次数耗尽而 failed 的评审重新入队（重置 retry_count）。
	h.POST("/api/v1/reviews/:id/requeue", func(ctx context.Context, c *app.RequestContext) {
		reviewID := c.Param("id")
		if err := repos.Review.RequeueFromDeadLetter(ctx, reviewID); err != nil {
			common.Fail(ctx, c, err)
			return
		}
		common.Success(c, map[string]any{"ok": true, "review_id": reviewID})
	})
}

// registerDelegateRoutes 注册 OCR v1.7.11+ delegate 子命令的 HTTP 端点（host-agent 范式）。
//
// 不走 thrift IDL：delegate 是可选能力，作为手写路由暴露避免大规模 generated 代码手改。
// - POST /api/v1/delegate/preview：返回将参与评审的文件列表 + mode/from/to/merge_base/total_*/Files
// - POST /api/v1/delegate/rule：返回按内容分组的解析规则 markdown
//
// 端点需要 REVIEW_TOKEN 鉴权（与其它 /api/v1/* 端点一致，由全局 auth middleware 强制）。
func registerDelegateRoutes(h *server.Hertz) {
	h.POST("/api/v1/delegate/preview", reviewHandler.DelegatePreviewHandler)
	h.POST("/api/v1/delegate/rule", reviewHandler.DelegateRuleHandler)
}

func registerSettingsExtraRoutes(h *server.Hertz) {
	h.GET("/api/v1/settings/rules/packages/:id/history", settingsHandler.ListRulePackageVersions)
	h.POST("/api/v1/settings/rules/packages/:id/history/:version/restore", settingsHandler.RestoreRulePackageVersion)
	h.GET("/api/v1/settings/ocr/tool-packages/:id/history", settingsHandler.ListToolPackageVersions)
	h.POST("/api/v1/settings/ocr/tool-packages/:id/history/:version/restore", settingsHandler.RestoreToolPackageVersion)
	h.POST("/api/v1/settings/ocr/viewer/locate", settingsHandler.LocateViewerSession)
}

func registerDiagnosticsRoutes(h *server.Hertz) {
	h.GET("/api/v1/diagnostics", diagnosticsHandler.Snapshot)
}

func registerFeedbackRoutes(h *server.Hertz) {
	h.GET("/api/v1/reviews/:id/findings/feedback", feedbackHandler.ListFeedback)
	h.POST("/api/v1/reviews/:id/findings/:index/feedback", feedbackHandler.UpsertFeedback)
	// 按指纹批量查历史反馈，供跨 review 误报抑制。
	h.POST("/api/v1/findings/feedback/lookup", feedbackHandler.ListFeedbackByFingerprints)
}

func registerViewerProxyRoute(h *server.Hertz, viewerMgr *ocrcore.Viewer, authToken string) {
	h.GET("/api/v1/settings/ocr/viewer/proxy/*path", viewerProxyHandler(viewerMgr, authToken))
}

// proxiedResponseHeaders 是 viewer 反向代理允许转发给客户端的上游响应头白名单。
// 刻意排除 Set-Cookie / Location / Server / Transfer-Encoding 等，避免响应头注入。
var proxiedResponseHeaders = []string{
	"Content-Type",
	"Content-Length",
	"Content-Encoding",
	"Content-Language",
	"Cache-Control",
	"ETag",
	"Last-Modified",
	"Expires",
	"Pragma",
	"Vary",
}

func viewerProxyHandler(viewerMgr *ocrcore.Viewer, authToken string) app.HandlerFunc {
	proxyClient := &http.Client{Timeout: 30 * time.Second}
	const maxProxyBody = 10 << 20 // 10 MB

	return func(ctx context.Context, c *app.RequestContext) {
		// iframe 无法携带 Authorization 头，改用 token 查询参数鉴权。
		if authToken != "" {
			t := string(c.Query("token"))
			if subtle.ConstantTimeCompare([]byte(t), []byte(authToken)) != 1 {
				c.SetStatusCode(http.StatusUnauthorized)
				_, _ = c.WriteString("unauthorized")
				return
			}
		}

		if viewerMgr == nil {
			c.SetStatusCode(http.StatusServiceUnavailable)
			return
		}
		status, statusErr := viewerMgr.ViewerStatus(ctx)
		if statusErr != nil {
			logx.Warnf("viewer proxy: get viewer status failed: %v", statusErr)
		}
		if statusErr != nil || !status.Running || status.Addr == "" {
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
		// 仅转发安全的响应头：此前原样拷贝上游全部响应头，
		// Set-Cookie / Location / Server 等会被注入到客户端（响应头注入）。
		for _, key := range proxiedResponseHeaders {
			if values := resp.Header.Values(key); len(values) > 0 {
				for _, v := range values {
					c.Response.Header.Add(key, v)
				}
			}
		}
		// 强制允许同源 iframe 嵌入，覆盖上游可能返回的限制头。
		c.Response.Header.Set("X-Frame-Options", "SAMEORIGIN")
		body, err := io.ReadAll(io.LimitReader(resp.Body, maxProxyBody))
		if err != nil {
			c.SetStatusCode(http.StatusBadGateway)
			return
		}
		c.SetStatusCode(resp.StatusCode)
		_, _ = c.Write(body)
	}
}
