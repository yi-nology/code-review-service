package mcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/yi-nology/code-review-service/internal/app"
	tcdb "github.com/yi-nology/code-review-service/internal/repository/postgres/testutil"

	"github.com/testcontainers/testcontainers-go"
)

// TestMCPSSEEndToEnd 端到端：起 testcontainers Postgres → 启动 mcp-server 二进制（HTTP 模式）→
// 走 HTTP initialize + tools/list + tools/call(create_review)。
//
// 覆盖：
//  1. HTTP server 能在 :8082 listen（端口从 --port 注入避免与其他 e2e 撞）
//  2. /healthz 不要求鉴权
//  3. /mcp 缺 Authorization 头返 401
//  4. 带正确 Bearer token 的 initialize 走通
//  5. tools/list 返 13 个 tool
//  6. create_review 错误参数走通
//
// HTTP transport: MCP 2025-03 spec，streamable_http。client 跟 server 之间用
// POST /mcp 单向请求-响应（带 session-id header 标识）；本测试只覆盖 stateless 部分。
func TestMCPHTTPEndToEnd(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	pg, _, err := tcdb.StartPostgres(ctx)
	if err != nil {
		if isDockerUnavailable(err) {
			t.Skip(dockerSkip)
		}
		t.Fatalf("start postgres: %v", err)
	}
	t.Cleanup(func() { _ = pg.Terminate(context.Background()) })

	bin := buildMCPServerBinary(t)
	port := freePort(t)

	cmd := exec.Command(bin,
		fmt.Sprintf("--port=%d", port),
	)
	cmd.Env = append(os.Environ(),
		"DATABASE_URL="+dbConnString(t, pg, ctx),
		"AUTH_INSECURE=true",
		"MCP_TOKEN=test-e2e-token-12345",
		fmt.Sprintf("MCP_PORT=%d", port),
	)
	cmd.Stderr = os.Stderr // 让 fx 启动日志直接打到测试 stderr
	if err := cmd.Start(); err != nil {
		t.Fatalf("start mcp-server: %v", err)
	}
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	})

	// 等 server listen。
	base := fmt.Sprintf("http://127.0.0.1:%d", port)
	waitForHealthz(t, base, 30*time.Second)

	t.Run("healthz no auth", func(t *testing.T) {
		resp, err := http.Get(base + "/healthz")
		if err != nil {
			t.Fatalf("get /healthz: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Errorf("expected 200, got %d", resp.StatusCode)
		}
	})

	t.Run("/mcp without auth → 401", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodPost, base+"/mcp", strings.NewReader("{}"))
		req.Header.Set("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("post: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 401 {
			t.Errorf("expected 401, got %d", resp.StatusCode)
		}
		body, _ := io.ReadAll(resp.Body)
		if !strings.Contains(string(body), "unauthorized") {
			t.Errorf("expected 'unauthorized' in body, got %s", body)
		}
	})

	t.Run("initialize with auth", func(t *testing.T) {
		resp, _ := callMCP(t, base, "test-e2e-token-12345", 1, "initialize", map[string]any{
			"protocolVersion": "2025-06-18",
			"capabilities":    map[string]any{},
			"clientInfo":      map[string]any{"name": "sse-e2e-test", "version": "1.0.0"},
		})
		var init struct {
			Result struct {
				ServerInfo struct {
					Name string `json:"name"`
				} `json:"serverInfo"`
			} `json:"result"`
		}
		if err := json.Unmarshal(resp, &init); err != nil {
			t.Fatalf("unmarshal: %v\n%s", err, resp)
		}
		if !strings.Contains(init.Result.ServerInfo.Name, "review-service") {
			t.Errorf("expected server name, got %q", init.Result.ServerInfo.Name)
		}
	})

	t.Run("tools/list with auth", func(t *testing.T) {
		// initialize 已经建立 session,后续请求必须带 session id (streamable_http spec)。
		// 简化: 重新调 initialize 拿 session, 然后用同 session 调 tools/list。
		_, sessionID := initializeForSession(t, base, "test-e2e-token-12345", 2)
		resp, _ := callMCPWithSession(t, base, "test-e2e-token-12345", sessionID, 3, "tools/list", map[string]any{})
		var list struct {
			Result struct {
				Tools []struct {
					Name string `json:"name"`
				} `json:"tools"`
			} `json:"result"`
		}
		if err := json.Unmarshal(resp, &list); err != nil {
			t.Fatalf("unmarshal: %v\n%s", err, resp)
		}
		if len(list.Result.Tools) != 13 {
			t.Errorf("expected 13 tools, got %d", len(list.Result.Tools))
		}
	})

	t.Run("create_review invalid_param", func(t *testing.T) {
		_, sessionID := initializeForSession(t, base, "test-e2e-token-12345", 4)
		resp, _ := callMCPWithSession(t, base, "test-e2e-token-12345", sessionID, 5, "tools/call", map[string]any{
			"name":      "create_review",
			"arguments": map[string]any{"provider": "github", "repo": "foo/bar"}, // 缺 head_sha
		})
		if cat := assertToolError(t, resp, ErrCategoryInvalidParam); cat == "" {
			t.Errorf("expected invalid_param, got: %s", resp)
		}
	})
}

// callMCP 走 HTTP POST /mcp 发 JSON-RPC 消息（MCP 2025-03 streamable_http transport）。
//
// 返回: (response body, Mcp-Session-Id header)
// session id 可能在 initialize 时由服务端生成,后续请求必须带;本函数做透明传递。
func callMCP(t *testing.T, base, token string, id int, method string, params any) ([]byte, string) {
	return callMCPWithSession(t, base, token, "", id, method, params)
}

// callMCPWithSession 同 callMCP 但显式带 session id。sessionID 为空表示不开 session。
func callMCPWithSession(t *testing.T, base, token, sessionID string, id int, method string, params any) ([]byte, string) {
	t.Helper()
	body, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"method":  method,
		"params":  params,
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	req, _ := http.NewRequest(http.MethodPost, base+"/mcp", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("Authorization", "Bearer "+token)
	if sessionID != "" {
		req.Header.Set("Mcp-Session-Id", sessionID)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	defer resp.Body.Close()
	gotSession := resp.Header.Get("Mcp-Session-Id")
	if resp.StatusCode != 200 {
		raw, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, raw)
	}

	// streamable_http 响应：可能是 application/json 或 text/event-stream。
	// 简化测试：读全部 body 解析 JSON-RPC 响应。
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	ct := resp.Header.Get("Content-Type")
	if strings.Contains(ct, "text/event-stream") {
		sc := bufio.NewScanner(bytes.NewReader(raw))
		for sc.Scan() {
			line := sc.Text()
			if strings.HasPrefix(line, "data: ") {
				return []byte(strings.TrimPrefix(line, "data: ")), gotSession
			}
		}
		t.Fatalf("no data: line in SSE response: %s", raw)
	}
	return raw, gotSession
}

// initializeForSession 调 initialize 拿服务端分配的 Mcp-Session-Id。
// spec: initialize 响应中服务端可选返回 Mcp-Session-Id header,后续请求必须带。
func initializeForSession(t *testing.T, base, token string, id int) ([]byte, string) {
	return callMCP(t, base, token, id, "initialize", map[string]any{
		"protocolVersion": "2025-06-18",
		"capabilities":    map[string]any{},
		"clientInfo":      map[string]any{"name": "sse-e2e-test", "version": "1.0.0"},
	})
}

// waitForHealthz 轮询 /healthz 直到 200 或超时。给 fx 启动 + DB 连接 + HTTP listen 留时间。
func waitForHealthz(t *testing.T, base string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		resp, err := http.Get(base + "/healthz")
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == 200 {
				return
			}
		}
		time.Sleep(500 * time.Millisecond)
	}
	t.Fatalf("healthz not ready after %s", timeout)
}

// freePort 拿一个空闲端口（避免本测试跟其他 e2e 撞 8082）。
func freePort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()
	return ln.Addr().(*net.TCPAddr).Port
}

// 防止 import unused：app / testcontainers 仅在函数签名 / 编译期约束里出现,
// 写两个 blank var 让 import 不被剔除。helper 跟常量定义见 e2e_helpers_test.go。
var _ app.ReviewService
var _ testcontainers.Container
