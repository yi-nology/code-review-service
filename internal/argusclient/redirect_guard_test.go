// redirect_guard_test.go 同源重定向钳制（GO-2026-6911 自持缓解）的回归钉：
// 跨源 Location 必须拒绝且请求不得离开本机（凭证不外带），同源 Location 放行。
package argusclient

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// 同源判定：scheme+主机 归一（默认端口折叠、大小写不敏感），端口/协议/主机任一不同即跨源。
func TestSameOrigin(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{"http://argus.example", "HTTP://ARGUS.EXAMPLE", true},
		{"http://argus.example:80", "http://argus.example", true},
		{"https://argus.example:443", "https://argus.example", true},
		{"http://argus.example:8080", "http://argus.example", false},
		{"http://argus.example", "https://argus.example", false},
		{"http://argus.example", "http://evil.example", false},
		{"http://127.0.0.1:1", "http://127.0.0.1:2", false}, // httptest 双 server 场景
		{"http://argus.example/sub/path", "http://argus.example/other", true},
	}
	for _, tc := range cases {
		ua, ub := mustURL(t, tc.a), mustURL(t, tc.b)
		if got := sameOrigin(ua, ub); got != tc.want {
			t.Errorf("sameOrigin(%q,%q)=%v want %v", tc.a, tc.b, got, tc.want)
		}
	}
}

func mustURL(t *testing.T, raw string) *url.URL {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	return u
}

// REST 通道：同源 302（LB 路径改写类）跟随并拿到最终响应。
func TestRedirectGuardAllowsSameOrigin(t *testing.T) {
	c := newFakeArgus(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/tasks/42":
			http.Redirect(w, r, "/tasks/42/real", http.StatusFound)
		case "/tasks/42/real":
			_, _ = w.Write([]byte(envelope(`{"taskId":"42","status":"done"}`)))
		default:
			http.NotFound(w, r)
		}
	})
	st, err := c.GetTask(context.Background(), "42")
	if err != nil {
		t.Fatal(err)
	}
	if st.TaskID != "42" || st.Status != "done" {
		t.Fatalf("同源重定向后应拿到最终响应: %+v", st)
	}
}

// REST 通道：跨源 302 必须拒绝，且重定向目标收不到任何请求（凭证不外带）。
func TestRedirectGuardBlocksCrossOrigin(t *testing.T) {
	var evilHits int
	var evilAuth string
	evil := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		evilHits++
		evilAuth = r.Header.Get("Authorization")
	}))
	defer evil.Close()

	argus := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, evil.URL+"/steal", http.StatusFound)
	}))
	defer argus.Close()
	c := New(argus.URL, "secret-bearer")

	_, err := c.GetTask(context.Background(), "42")
	if err == nil {
		t.Fatal("跨源重定向应被拒绝")
	}
	if !errors.Is(err, ErrRedirectBlocked) {
		t.Fatalf("错误应可判定为 ErrRedirectBlocked: %v", err)
	}
	if evilHits != 0 {
		t.Fatalf("重定向目标不应收到任何请求, 实际 %d 次", evilHits)
	}
	if evilAuth != "" {
		t.Fatalf("凭证不应外带: %q", evilAuth)
	}
}

// REST 通道：非 http(s) scheme 的 Location（file:// 等）同样拒绝（scheme 不同源）。
func TestRedirectGuardBlocksNonHTTPScheme(t *testing.T) {
	var fileHits int
	argus := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fileHits++
		http.Redirect(w, r, "file:///etc/passwd", http.StatusFound)
	}))
	defer argus.Close()
	c := New(argus.URL, "secret-bearer")

	_, err := c.GetTask(context.Background(), "42")
	if !errors.Is(err, ErrRedirectBlocked) {
		t.Fatalf("file:// Location 应被拒绝: %v", err)
	}
	if fileHits != 1 {
		t.Fatalf("仅应有初始请求, 实际 %d 次", fileHits)
	}
}

// 同源但超过跳数上限的重定向链同样拒绝（CheckRedirect 非空后 stdlib 10 跳默认上限失效）。
func TestRedirectGuardBlocksRedirectLoop(t *testing.T) {
	argus := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/loop", http.StatusFound)
	}))
	defer argus.Close()
	c := New(argus.URL, "tok")

	_, err := c.GetTask(context.Background(), "42")
	if !errors.Is(err, ErrRedirectBlocked) {
		t.Fatalf("同源超跳数也应拒绝: %v", err)
	}
	if !strings.Contains(err.Error(), fmt.Sprintf("%d 跳", maxSameOriginRedirects)) {
		t.Fatalf("错误应说明跳数上限: %v", err)
	}
}

// MCP 通道：AskQuestion 注入 transport 的 client 必须带同源钳制且不带整体
// Timeout（SSE 长流语义），跨源 Location 拒绝且请求不出本机。
func TestMCPHTTPClientGuard(t *testing.T) {
	var evilHits int
	evil := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		evilHits++
	}))
	defer evil.Close()

	argus := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, evil.URL+"/mcp/steal", http.StatusFound)
	}))
	defer argus.Close()
	c := New(argus.URL, "secret-bearer")

	cl := c.mcpHTTPClient()
	if cl.Timeout != 0 {
		t.Fatalf("MCP client 不应带整体 Timeout（SSE 长流）: %v", cl.Timeout)
	}
	if cl.CheckRedirect == nil {
		t.Fatal("MCP client 必须带 CheckRedirect 钳制")
	}

	req, err := http.NewRequest(http.MethodPost, argus.URL+"/mcp", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer secret-bearer")
	resp, err := cl.Do(req)
	if err == nil {
		resp.Body.Close()
		t.Fatal("跨源重定向应被拒绝")
	}
	if !errors.Is(err, ErrRedirectBlocked) {
		t.Fatalf("错误应可判定为 ErrRedirectBlocked: %v", err)
	}
	if evilHits != 0 {
		t.Fatalf("重定向目标不应收到任何请求, 实际 %d 次", evilHits)
	}
}

// MCP 通道：同源 Location 正常跟随（argus 部署在反向代理后属正常拓扑）。
func TestMCPHTTPClientAllowsSameOrigin(t *testing.T) {
	argus := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/mcp":
			http.Redirect(w, r, "/mcp/", http.StatusFound)
		case "/mcp/":
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","result":{},"id":1}`))
		}
	}))
	defer argus.Close()
	c := New(argus.URL, "tok")

	req, err := http.NewRequest(http.MethodPost, argus.URL+"/mcp", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := c.mcpHTTPClient().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("同源重定向应跟随到 200, 实际 %d", resp.StatusCode)
	}
}
