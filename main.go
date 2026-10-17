package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/yi-nology/code-review-service/internal/config"
	"github.com/oekit/bootstrap/bootstrap"
	nacosbs "github.com/oekit/bootstrap/nacos"
	obs "github.com/oekit/bootstrap/obs"
	"github.com/oekit/logx"

	_ "github.com/yi-nology/go-git-platform/backends/all" // 注册所有平台后端（v0.32.0 起 provider 不再自动注册）
)

func init() {
	// ekit v0.17.4 hlog guard: 显式切 slog, 62 个文件 package-level logx.* 走 slog
	logx.SetDefault(logx.NewSlogLogger("review-service"))
}

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

	// ekit bootstrap.Run(Config) 便捷入口在 oekit 拆分时移除，此处展开等价组合
	// （与旧 Run 语义一致：ServiceName + Metrics + ShutdownTimeout + Tracing + NNS）。
	opts := []bootstrap.BootstrapperOption{
		bootstrap.WithServiceName("review-service"),
		obs.WithMetrics(),
		bootstrap.WithShutdownTimeout(defaultShutdownTimeout),
	}
	if tc := cfg.GetTracingConfig(); tc != nil {
		opts = append(opts, obs.WithTracing(*tc))
	}
	if cfg.Nacos.ServiceName != "" {
		opts = append(opts, nacosbs.WithNNS(cfg.Nacos, cfg.Port, nil))
	}
	b, berr := bootstrap.New(opts...)
	if berr != nil {
		_, _ = fmt.Fprintf(os.Stderr, "bootstrap init failed: %v\n", berr)
		os.Exit(1)
	}
	if err := b.Run(func(app *bootstrap.App) error {
		return runApp(cfg, app)
	}); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "bootstrap failed: %v\n", err)
		os.Exit(1)
	}
}
