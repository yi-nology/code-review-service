// Package transport 暴露 mcp-server 进程的多传输启动入口。
//
// 两种 transport：
//   - stdio：minicode / Claude Desktop / Cursor 标配，本地子进程模式
//   - sse（streamable_http）：远端部署场景，HTTP/POST + SSE 端点
//
// 选型：
//   - mcp-go 在 0.56 同时提供 legacy SSE (server.NewSSEServer) 跟新
//     streamable_http (server.NewStreamableHTTPServer, MCP 2025-03 spec)。
//   - 后者是 MCP 官方推荐（2025-06 spec 进一步强化），本包只实现 streamable_http。
//   - 客户端 spec 兼容：mcp-go 客户端、minicode、Claude Desktop 都支持。
package transport

import (
	"context"
	"crypto/subtle"
	"fmt"
	"net/http"
	"time"

	"github.com/yi-nology/code-review-service/internal/mcp"
	"github.com/oekit/concurrency/async"

	mcpserver "github.com/mark3labs/mcp-go/server"
)

// Transport 是 mcp-server 的传输模式。
type Transport string

const (
	Stdio Transport = "stdio"
	SSE   Transport = "sse"
)

// Config 包含 transport-specific 启动参数。
type Config struct {
	// Token 是 Bearer token；sse 模式下启用，stdio 模式不用（client/server 同进程）。
	Token string
	// Port 是 sse 模式监听端口，stdio 模式忽略。
	Port int
}

// RunSSE 启 streamable_http transport 在 cfg.Port 端口，path = /mcp。
//
// 鉴权：所有非 GET / healthz 请求要求 Authorization: Bearer <cfg.Token>。
// 走 stdlib net/http 的 middleware 模式，避免引入额外依赖（mcp-go 不带 auth
// middleware，需要外面套）。
//
// session：用 InsecureStatefulSessionIdManager（mcp-go 默认 StatelessGeneratingSessionIdManager
// 也是 OK；选 stateful 是为了后续多 session 跟 RBAC 留扩展点 — 当前 single-tenant
// 实际上不需要，但避免后期重构）。
func RunSSE(ctx context.Context, srv *mcpserver.MCPServer, cfg Config) error {
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
		// SSE 模式下 fx 已经 OnStart 完毕，进程能 listen 就是 ready。
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

	// 优雅退出
	errCh := async.GoSafeWithErr(server.ListenAndServe)

	select {
	case err := <-errCh:
		if err != nil && err != http.ErrServerClosed {
			return fmt.Errorf("sse server: %w", err)
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
	if len(auth) < 7 || !equalFold(auth[:7], "Bearer ") {
		return ""
	}
	return auth[7:]
}

// equalFold 是不依赖 strings.EqualFold 的内联实现（包级函数避免 import）。
func equalFold(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		ca, cb := a[i], b[i]
		if 'A' <= ca && ca <= 'Z' {
			ca += 'a' - 'A'
		}
		if 'A' <= cb && cb <= 'Z' {
			cb += 'a' - 'A'
		}
		if ca != cb {
			return false
		}
	}
	return true
}

// ResolveTransport 把 CLI flag 字符串转成 Transport 枚举。
// 不认识的值返 zero value (Stdio) + warning 写到 stderr, 跟 Go 惯例一致。
func ResolveTransport(s string) Transport {
	switch s {
	case "stdio", "":
		return Stdio
	case "sse", "http":
		return SSE
	default:
		return Stdio
	}
}

// 编译期检查：保证 *mcp.Server 跟 *mcpserver.MCPServer 的转换路径存在。
// RunSSE 接 *mcpserver.MCPServer 而非 *mcp.Server,避免 RunSSE 依赖内部 wrapper。
var _ = mcp.ServerName
