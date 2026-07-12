// cmd/mcp-server 是 review-service 的 MCP 端点（独立进程）。
//
// 与主 HTTP 服务的区别：
//   - 不起 Hertz：stdio 模式从 stdin 读 JSON-RPC 写到 stdout；SSE 模式由 --transport=sse 启 HTTP。
//   - 复用 service 层：fx 拉 DatabaseModule / RepositoryModule / ProviderModule /
//     RunnerModule / OCRModule / LLMModule / WritebackModule / NotificationModule /
//     CostModule / AgentModule / WorkflowModule / ReviewModule；
//     不拉 ServerModule / WebhookModule / HandlerModule / WorkerModule /
//     PreflightModule / LLMProbeModule / MigrateModule / MigrateRulesModule /
//     CleanupModule — 这些是 HTTP 路由、webhook 接收、后台 worker、迁移专用，
//     MCP 端点不需要。
//   - 鉴权独立：MCP_TOKEN env 优先，fallback REVIEW_TOKEN。
//
// 启动方式（stdio, minicode 本地场景）：
//
//	REVIEW_TOKEN=xxx DATABASE_URL=postgres://... go run ./cmd/mcp-server
//	# 或
//	REVIEW_TOKEN=xxx DATABASE_URL=postgres://... ./bin/mcp-server
//
// minicode 端 mcp_config.json 示例：
//
//	{
//	  "mcpServers": {
//	    "review-service": {
//	      "command": "/abs/path/to/bin/mcp-server",
//	      "env": {
//	        "REVIEW_TOKEN": "xxx",
//	        "DATABASE_URL": "postgres://..."
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
	"strconv"
	"syscall"
	"time"

	"github.com/yi-nology/code-review-service/internal/config"
	mcpserver "github.com/yi-nology/code-review-service/internal/mcp"
	"github.com/yi-nology/code-review-service/internal/mcp/transport"
	"github.com/yi-nology/code-review-service/internal/modules"
	"github.com/yi-nology/code-review-service/internal/observability/redact"

	"github.com/oekit/logx"
	mcpruntime "github.com/mark3labs/mcp-go/server"
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
	transportFlag := flag.String("transport", "stdio", "transport: stdio (default) or sse (streamable_http)")
	portFlag := flag.Int("port", 8082, "sse mode listen port (ignored in stdio mode)")
	flag.Parse()

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

	// 选 transport。
	tr := transport.ResolveTransport(*transportFlag)
	logx.Infof("mcp server transport: %s", tr)

	// SSE 模式下,port 可以从 --port / MCP_PORT env 覆盖。
	if tr == transport.SSE {
		if p := os.Getenv("MCP_PORT"); p != "" {
			if v, err := strconv.Atoi(p); err == nil && v > 0 {
				*portFlag = v
			}
		}
	}

	var runErr error
	switch tr {
	case transport.Stdio:
		runErr = runStdio(mcpSrv)
	case transport.SSE:
		runErr = runSSE(startCtx, mcpSrv, mcpToken, *portFlag)
	default:
		runErr = fmt.Errorf("unknown transport: %s", *transportFlag)
	}
	if runErr != nil {
		logx.Warnf("mcp server %s ended: %v", tr, runErr)
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

// runStdio 跑 stdio 传输，捕获 SIGTERM / SIGINT 触发优雅退出。
//
// mcp-go 的 server.ServeStdio 内部已处理 SIGINT/SIGTERM，
// 但我们额外包一层是为了在退出前调 fx.Stop，让 DB 连接 / OCR 子进程 /
// progress goroutine 都能正确清理。
func runStdio(srv *mcpserver.Server) error {
	// 独立信号通道：stdin 关闭 vs 用户按 Ctrl+C 都能触发退出。
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)

	errCh := make(chan error, 1)
	go func() {
		errCh <- mcpruntime.ServeStdio(srv.Runtime())
	}()

	select {
	case sig := <-sigCh:
		logx.Infof("received signal %s, shutting down mcp server", sig)
		// ServeStdio 内部会处理信号；这里不需要额外动作，让它从 stdin EOF 退出。
		// 等待 ServeStdio 自然返回（最多 5s），避免阻塞 main。
		select {
		case <-errCh:
		case <-time.After(5 * time.Second):
			logx.Warn("stdio server did not exit in 5s after signal; force exit")
		}
		return nil
	case err := <-errCh:
		// stdin EOF 是正常退出（minicode 关闭 IDE）。
		return err
	}
}

// runSSE 跑 streamable_http transport。ctx 由 main 传入,signal 触发 cancel。
func runSSE(ctx context.Context, srv *mcpserver.Server, token string, port int) error {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)

	errCh := make(chan error, 1)
	go func() {
		errCh <- transport.RunSSE(ctx, srv.Runtime(), transport.Config{
			Token: token,
			Port:  port,
		})
	}()

	select {
	case sig := <-sigCh:
		logx.Infof("received signal %s, shutting down sse server", sig)
		// RunSSE 在 ctx.Done() 时会优雅关闭 server。等其返回。
		select {
		case err := <-errCh:
			return err
		case <-time.After(15 * time.Second):
			return fmt.Errorf("sse server did not exit in 15s after signal")
		}
	case err := <-errCh:
		return err
	}
}
