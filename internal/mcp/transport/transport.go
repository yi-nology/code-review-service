// Package transport 暴露 mcp-server 进程的 HTTP 传输启动入口。
//
// 选型：
//   - mcp-go 在 0.56 同时提供 legacy SSE (server.NewSSEServer) 跟新
//     streamable_http (server.NewStreamableHTTPServer, MCP 2025-03 spec)。
//   - 选 streamable_http：MCP 官方推荐（2025-06 spec 进一步强化），本包只实现这一种。
//   - 历史 stdio transport 已删除（mcp-server 进程只服务远端 HTTP 调用场景；
//     IDE 本地子进程 stdio 模式由 IDE 端走 HTTP 远端接入）。
//   - 客户端 spec 兼容：mcp-go 客户端、minicode、Claude Desktop、Cursor 都支持。
package transport

import (
	"strings"
	"context"
	"crypto/subtle"
	"fmt"
	"net/http"
	"time"

	"github.com/oekit/concurrency/async"

	mcpserver "github.com/mark3labs/mcp-go/server"
)

// Config 包含 HTTP transport 启动参数。
type Config struct {
	// Token 是 Bearer token；空值时关闭鉴权（dev 模式 + AUTH_INSECURE=true）。
	Token string
	// Port 是 HTTP 监听端口，<=0 时默认 8082。
	Port int
}

// RunHTTP 启 streamable_http transport 在 cfg.Port 端口，path = /mcp。
//
// 鉴权：所有非 /healthz / /readyz 请求要求 Authorization: Bearer <cfg.Token>。
// 走 stdlib net/http 的 middleware 模式，避免引入额外依赖（mcp-go 不带 auth
// middleware，需要外面套）。
//
// session：用 InsecureStatefulSessionIdManager（mcp-go 默认 StatelessGeneratingSessionIdManager
// 也是 OK；选 stateful 是为了后续多 session 跟 RBAC 留扩展点 — 当前 single-tenant
// 实际上不需要，但避免后期重构）。
func RunHTTP(ctx context.Context, srv *mcpserver.MCPServer, cfg Config) error {
	if cfg.Port <= 0 {
		cfg.Port = 8082
	}
	addr := fmt.Sprintf(":%d", cfg.Port)

	httpSrv := mcpserver.NewStreamableHTTPServer(srv,
		mcpserver.WithEndpointPath("/mcp"),
		mcpserver.WithStateful(true),
	)

	mux := http.NewServeMux()
	mux.Handle("/mcp", bearerAuth(httpSrv, cfg.Token))
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		// HTTP 模式下 fx 已经 OnStart 完毕，进程能 listen 就是 ready。
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ready"}`))
	})

	server := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
		// SSE 长连接需要 MaxHeaderBytes 放宽,默认 1MB 够用,不动。
	}

	errCh := make(chan error, 1)
	async.GoSafeWithErrChannel(errCh, func() error {
		return server.ListenAndServe()
	})

	select {
	case err := <-errCh:
		if err != nil && err != http.ErrServerClosed {
			return fmt.Errorf("http server: %w", err)
		}
		return nil
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return server.Shutdown(shutdownCtx)
	}
}

// bearerAuth 是 Bearer token 中间件。
//
// 鉴权语义：
//   - 路径是 /healthz /readyz 跳过鉴权（k8s probe 场景）
//   - 其他路径要求 Authorization: Bearer <token>，token 不匹配返 401
//   - 走 constant-time compare 防 timing attack
//   - 缺失 Authorization 头也返 401（不返 403，统一处理）
func bearerAuth(next http.Handler, token string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// health 路径跳过鉴权,k8s 探针不携带 Authorization。
		if r.URL.Path == "/healthz" || r.URL.Path == "/readyz" {
			next.ServeHTTP(w, r)
			return
		}
		// token 为空时关闭鉴权（dev 模式 + AUTH_INSECURE=true）。
		if token == "" {
			next.ServeHTTP(w, r)
			return
		}
		got := bearerFromRequest(r)
		if subtle.ConstantTimeCompare([]byte(got), []byte(token)) != 1 {
			w.Header().Set("WWW-Authenticate", `Bearer realm="mcp-server"`)
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":"unauthorized"}`))
			return
		}
		next.ServeHTTP(w, r)
	})
}

func bearerFromRequest(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if len(auth) < 7 || !strings.EqualFold(auth[:7], "Bearer ") {
		return ""
	}
	return auth[7:]
}

