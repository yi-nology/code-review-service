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
		modules.WorkerModule,
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
