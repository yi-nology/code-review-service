package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/yi-nology/code-review-service/internal/config"
	"github.com/oekit/bootstrap"
)

const defaultShutdownTimeout = 30 * time.Second

func main() {
	// PR-19 修复：原实现把 flag.String("env", ...) 放在包级全局变量，
	// init 顺序敏感、测试时 flag.Parse 互相干扰。改为 main 显式注册。
	env := flag.String("env", "local", "environment: local, dev, prod")
	flag.Parse()

	config.SetCurrentEnv(*env)
	cfg := config.LoadWithEnv(*env)
	if err := cfg.Validate(config.CurrentEnv()); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}

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
