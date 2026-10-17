package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/yi-nology/code-review-service/internal/config"
	"github.com/yi-nology/code-review-service/internal/modules"
	"github.com/yi-nology/code-review-service/internal/observability/redact"

	"github.com/oekit/bootstrap/bootstrap"
	"github.com/oekit/logx"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/common/hlog"
	"go.uber.org/fx"
	"go.uber.org/fx/fxevent"
)

func init() {
	// PR-08 修复：把全局 logx 包装为自动脱敏 logger，
	// 拦截 logx.Info/Warn/Error 与 %v %w 错误中的敏感字段（Bearer / api_key / sk- / ghp_ / ...）。
	logx.SetDefault(redact.NewLogxLogger(logx.Default()))
}

func runApp(cfg *config.Config, app *bootstrap.App) error {
	h := createServer(cfg)

	fxApp := fx.New(
		fx.Supply(cfg, h),
		// 模块注册顺序的停机语义约束（fx 按注册逆序执行 OnStop）：
		//   DatabaseModule 最先注册 → 其 OnStop 最后执行（关库）。
		//   WorkerModule/LLMProbeModule 在其后注册 → 其 OnStop 先执行（drain in-flight / 停探活）。
		// 这保证了 worker 能在 DB 关闭前完成对 in-flight review 的落库。
		// 调整顺序时务必保持 WorkerModule 位于 DatabaseModule 之后。
		// CoreServiceModule：Database→Review 的共享装配（与 cmd/mcp-server 同源，
		// ReviewService 依赖变化只改 modules/core.go，防止 mcp 侧手工拷贝清单漂移）。
		modules.CoreServiceModule,
		modules.WebhookModule,
		modules.HandlerModule,
		modules.ServerModule,
		modules.PreflightModule,
		modules.LLMProbeModule,
		modules.WorkerModule,
		modules.MigrateModule,
		modules.MigrateRulesModule,
		modules.CleanupModule,
		fx.WithLogger(func() fxevent.Logger { return &fxevent.ConsoleLogger{W: os.Stderr} }),
	)

	ctx := app.Context()
	if err := fxApp.Start(ctx); err != nil {
		return fmt.Errorf("fx start: %w", err)
	}

	app.AddResource("fx", func(ctx context.Context) error {
		return fxApp.Stop(ctx)
	})

	hlog.Infof("review service starting on port %d", cfg.Port)
	return h.Run()
}

func createServer(cfg *config.Config) *server.Hertz {
	return server.Default(
		server.WithHostPorts(fmt.Sprintf(":%d", cfg.Port)),
		server.WithExitWaitTime(15*time.Second),
		// PR-06 修复：显式配置 body 大小与超时，防止大文件 / 慢客户端 DoS。
		// 16MB 足够容纳 settings PUT(rule package / tool package / custom provider)的最大合法负载；
		// webhook 端点在 handler 层另有 MaxPayloadBytes 限制。
		server.WithMaxRequestBodySize(16<<20),
		server.WithReadTimeout(30*time.Second),
		server.WithWriteTimeout(60*time.Second),
		server.WithIdleTimeout(120*time.Second),
	)
}
