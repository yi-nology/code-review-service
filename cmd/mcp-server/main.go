// cmd/mcp-server 是 review-service 的 MCP 端点（独立进程）。
//
// 与主 HTTP 服务的区别：
//   - 不起 Hertz：HTTP 模式由 transport.RunHTTP 启 streamable_http server。
//   - 复用 service 层：fx 拉 DatabaseModule / RepositoryModule / ProviderModule /
//     RunnerModule / OCRModule / LLMModule / WritebackModule / NotificationModule /
//     CostModule / AgentModule / WorkflowModule / ReviewModule；
//     不拉 ServerModule / WebhookModule / HandlerModule / WorkerModule /
//     PreflightModule / LLMProbeModule / MigrateModule / MigrateRulesModule /
//     CleanupModule — 这些是 HTTP 路由、webhook 接收、后台 worker、迁移专用，
//     MCP 端点不需要。
//   - 鉴权独立：MCP_TOKEN env 优先，fallback REVIEW_TOKEN。
//
// 启动方式：
//
//	REVIEW_TOKEN=xxx DATABASE_URL=postgres://... go run ./cmd/mcp-server
//	# 或
//	REVIEW_TOKEN=xxx DATABASE_URL=postgres://... ./bin/mcp-server
//
// 端口：默认 8082，--port 覆盖，或 MCP_PORT env 覆盖。
//
// Claude Desktop / Cursor / minicode 等 LLM 客户端接入示例（HTTP 模式）：
//
//	{
//	  "mcpServers": {
//	    "review-service": {
//	      "url": "http://localhost:8082/mcp",
//	      "headers": {
//	        "Authorization": "Bearer <MCP_TOKEN>"
//	      }
//	    }
//	  }
//	}
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"

	"syscall"
	"time"

	"github.com/yi-nology/code-review-service/internal/config"
	mcpserver "github.com/yi-nology/code-review-service/internal/mcp"
	"github.com/yi-nology/code-review-service/internal/mcp/transport"
	"github.com/yi-nology/code-review-service/internal/modules"
	"github.com/yi-nology/code-review-service/internal/observability/redact"

	"github.com/oekit/concurrency/async"
	"github.com/oekit/logx"
	"github.com/oekit/pkg/convert"
	"go.uber.org/fx"
	"go.uber.org/fx/fxevent"

	_ "github.com/yi-nology/git-platform-sdk/backends/all" // 注册所有平台后端（同主服务）
)

const (
	defaultShutdownTimeout = 30 * time.Second
	mcpServerName          = "review-service-mcp"
	mcpServerVersion       = "0.1.0"
)

// mcpSrv 由 fx.Invoke 闭包赋值；fx 启动后由 main 消费。
// 不在 fx.Provide 阶段赋初值是避免 nil 干扰 fx 注入检查。
var mcpSrv *mcpserver.Server

func init() {
	// 沿用主服务的 redact logger，避免 MCP 端日志泄露 LLM token / repo token。
	logx.SetDefault(redact.NewLogxLogger(logx.Default()))
}

func main() {
	env := flag.String("env", "local", "environment: local, dev, prod")
	portFlag := flag.Int("port", 8082, "http listen port")
	flag.Parse()

	// MCP_PORT env 覆盖 --port。
	if v := convert.Str2Int(os.Getenv("MCP_PORT"), 0); v > 0 {
		*portFlag = v
	}

	config.SetCurrentEnv(*env)
	cfg := config.LoadWithEnv(*env)
	if err := cfg.Validate(config.CurrentEnv()); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}

	// 独立 token：MCP_TOKEN 优先，fallback REVIEW_TOKEN。
	// 不强制要求：AUTH_INSECURE=true 跟主服务一样允许 dev 空 token 启动。
	mcpToken := os.Getenv("MCP_TOKEN")
	if mcpToken == "" {
		mcpToken = cfg.Auth.Token
	}
	if mcpToken == "" && !cfg.Auth.Insecure {
		fmt.Fprintf(os.Stderr,
			"neither MCP_TOKEN nor REVIEW_TOKEN configured (set one, or AUTH_INSECURE=true for dev)\n")
		os.Exit(1)
	}
	if mcpToken != "" {
		logx.Infof("mcp server: auth token loaded (length=%d, source=%s)",
			len(mcpToken), tokenSource())
	}

	fxApp := fx.New(
		fx.Supply(cfg),
		// 与主服务共享同一组基础设施 module，但跳过 HTTP / webhook / worker 相关。
		// 顺序约束：DatabaseModule 在最前 → ReviewModule 最后（依赖前置 module）。
		modules.DatabaseModule,
		modules.RepositoryModule,
		modules.ProviderModule,
		modules.RunnerModule,
		modules.OCRModule,
		modules.LLMModule,
		modules.WritebackModule,
		modules.NotificationModule,
		modules.CostModule,
		modules.AgentModule,
		modules.WorkflowModule,
		modules.ReviewModule,
		mcpserver.MCPModule,
		// fx.Invoke 把 *mcp.Server 从 fx container 提出来给 main 用：
		// fx.Provide 注册的对象不会自动暴露给 main 之外的代码，
		// 通过 Invoke + 闭包捕获是标准模式。
		fx.Invoke(func(s *mcpserver.Server) { mcpSrv = s }),
		fx.WithLogger(func() fxevent.Logger { return &fxevent.ConsoleLogger{W: os.Stderr} }),
	)

	startCtx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	if err := fxApp.Start(startCtx); err != nil {
		fmt.Fprintf(os.Stderr, "fx start: %v\n", err)
		os.Exit(1)
	}

	if mcpSrv == nil {
		fmt.Fprintf(os.Stderr, "*mcp.Server not resolved after fx start\n")
		_ = fxApp.Stop(context.Background())
		os.Exit(1)
	}

	// 启动 HTTP transport + signal 监听并发跑，err 走 channel。
	runCtx, runCancel := context.WithCancel(context.Background())
	defer runCancel()

	errCh := make(chan error, 1)
	async.GoSafeWithErrChannel(errCh, func() error {
		return transport.RunHTTP(runCtx, mcpSrv.Runtime(), transport.Config{
			Token: mcpToken,
			Port:  *portFlag,
		})
	})

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)

	select {
	case sig := <-sigCh:
		logx.Infof("received signal %s, shutting down mcp server", sig)
		runCancel() // 触发 RunHTTP 走 ctx.Done 路径 → server.Shutdown
		select {
		case err := <-errCh:
			if err != nil {
				logx.Warnf("mcp http server ended: %v", err)
			}
		case <-time.After(15 * time.Second):
			logx.Warn("mcp http server did not exit in 15s after signal")
		}
	case err := <-errCh:
		if err != nil {
			logx.Warnf("mcp http server ended: %v", err)
		}
	}

	// 优雅关闭 fx：先停 MCP，再关 DB。
	stopCtx, stopCancel := context.WithTimeout(context.Background(), defaultShutdownTimeout)
	defer stopCancel()
	if err := fxApp.Stop(stopCtx); err != nil {
		logx.Warnf("fx stop: %v", err)
	}
	logx.Info("mcp server stopped cleanly")
}

// tokenSource 仅用于日志 / 调试，提示用户 token 来自哪个 env。
func tokenSource() string {
	if os.Getenv("MCP_TOKEN") != "" {
		return "MCP_TOKEN"
	}
	return "REVIEW_TOKEN"
}
