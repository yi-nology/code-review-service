package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/yi-nology/code-review-service/internal/config"
	"github.com/yi-nology/code-review-service/internal/modules"

	"github.com/oekit/bootstrap"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/common/hlog"
	"go.uber.org/fx"
	"go.uber.org/fx/fxevent"
)

func runApp(cfg *config.Config, app *bootstrap.App) error {
	h := createServer(cfg)

	fxApp := fx.New(
		fx.Supply(cfg, h),
		// 模块注册顺序的停机语义约束（fx 按注册逆序执行 OnStop）：
		//   DatabaseModule 最先注册 → 其 OnStop 最后执行（关库）。
		//   WorkerModule/LLMProbeModule 在其后注册 → 其 OnStop 先执行（drain in-flight / 停探活）。
		// 这保证了 worker 能在 DB 关闭前完成对 in-flight review 的落库。
		// 调整顺序时务必保持 WorkerModule 位于 DatabaseModule 之后。
		modules.DatabaseModule,
		modules.RepositoryModule,
		modules.ProviderModule,
		modules.RunnerModule,
		modules.OCRModule,
		modules.LLMModule,
		modules.WritebackModule,
		modules.AgentModule,
		modules.WorkflowModule,
		modules.ReviewModule,
		modules.WebhookModule,
		modules.HandlerModule,
		modules.ServerModule,
		modules.PreflightModule,
		modules.LLMProbeModule,
		modules.WorkerModule,
		modules.MigrateModule,
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
	)
}
