package main

import (
	"time"

	"github.com/yi-nology/code-review-service/internal/config"
	"github.com/oekit/bootstrap"
)

const defaultShutdownTimeout = 30 * time.Second

func main() {
	cfg := config.Load()

	bootstrap.Run(bootstrap.Config{
		ServiceName:     "review-service",
		Port:            cfg.Port,
		Nacos:           cfg.Nacos,
		Tracing:         cfg.GetTracingConfig(),
		ShutdownTimeout: defaultShutdownTimeout,
	}, func(app *bootstrap.App) error {
		return runApp(cfg, app)
	})
}
