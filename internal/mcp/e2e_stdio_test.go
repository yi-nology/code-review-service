package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yi-nology/code-review-service/internal/app"
	tcdb "github.com/yi-nology/code-review-service/internal/repository/postgres/testutil"

	"github.com/testcontainers/testcontainers-go"
)

// docker 不可用时整套 stdio e2e 跳过；本地 / CI 没 docker 时不会假阳性。
var dockerSkip = errors.New("testcontainers: docker unavailable; skipping stdio e2e")

// TestMCPStdioEndToEnd 端到端：起 testcontainers Postgres → 启动 mcp-server 二进制 →
// 走 stdio JSON-RPC：initialize → notifications/initialized → listTools → tools/call(create_review)。
//
// 覆盖：
//   1. mcp-server 二进制能编译 + 启动 + fx wire 完整
//   2. initialize 握手返回 serverInfo.name=review-service-mcp
//   3. listTools 列出 13 个 tool（9 review + 4 platform）
//   4. create_review 缺 head_sha 时返回 invalid_param 错误
//   5. create_review 参数完整 + provider 不存在时返回 unsupported_provider 错误
func TestMCPStdioEndToEnd(t *testing.T) {
	// 启 Postgres。
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	pg, _, err := tcdb.StartPostgres(ctx)
	if err != nil {
		// docker 不可用：t.Skip 而不是 Fail，让本地没 docker 的人也能跑其他测试。
		if isDockerUnavailable(err) {
			t.Skip(dockerSkip)
		}
		t.Fatalf("start postgres: %v", err)
	}
	t.Cleanup(func() { _ = pg.Terminate(context.Background()) })

	// 编译 mcp-server。
	bin := buildMCPServerBinary(t)

	// 启动 mcp-server 子进程，连真 DB。
	cmd := exec.Command(bin)
	cmd.Env = append(os.Environ(),
		"DATABASE_URL="+dbConnString(t, pg, ctx),
		"AUTH_INSECURE=true",
		"MCPSERVER_DISABLE_OCR_PROBE=true", // 避免触发 OCR 子进程
	)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatalf("stdin pipe: %v", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("stdout pipe: %v", err)
	}
	stderr, _ := cmd.StderrPipe()
	if err := cmd.Start(); err != nil {
		t.Fatalf("start mcp-server: %v", err)
	}
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	})
	go func() {
		sc := bufio.NewScanner(stderr)
		for sc.Scan() {
			t.Logf("[mcp-server stderr] %s", sc.Text())
		}
	}()

	// 等等 fx 启动：fx 在 mcp-server 内 compile-time wired，container start 后 1-3s
	// 才能 ready。最稳的判据是发个 initialize 拿响应。
	// 这里加 5s 让 fx 完成 DB 连接 + OnStart hook。
	time.Sleep(5 * time.Second)

	// 1. initialize
	resp := sendJSONRPC(t, stdin, stdout, 1, "initialize", map[string]any{
		"protocolVersion": "2025-06-18",
		"capabilities":    map[string]any{},
		"clientInfo":      map[string]any{"name": "e2e-test", "version": "1.0.0"},
	})
	var init struct {
		Result struct {
			ServerInfo struct {
				Name    string `json:"name"`
				Version string `json:"version"`
			} `json:"serverInfo"`
			Capabilities map[string]any `json:"capabilities"`
		} `json:"result"`
		Error *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(resp, &init); err != nil {
		t.Fatalf("initialize unmarshal: %v\nraw: %s", err, resp)
	}
	if init.Error != nil {
		t.Fatalf("initialize returned error: code=%d msg=%s", init.Error.Code, init.Error.Message)
	}
	if !strings.Contains(init.Result.ServerInfo.Name, "review-service") {
		t.Errorf("expected server name contains 'review-service', got %q", init.Result.ServerInfo.Name)
	}
	t.Logf("mcp server connected: name=%s version=%s",
		init.Result.ServerInfo.Name, init.Result.ServerInfo.Version)

	// 2. notifications/initialized（MCP 协议要求）
	sendJSONRPC(t, stdin, stdout, 2, "notifications/initialized", map[string]any{})

	// 3. listTools
	resp = sendJSONRPC(t, stdin, stdout, 3, "tools/list", map[string]any{})
	var list struct {
		Result struct {
			Tools []struct {
				Name string `json:"name"`
			} `json:"tools"`
		} `json:"result"`
	}
	if err := json.Unmarshal(resp, &list); err != nil {
		t.Fatalf("listTools unmarshal: %v\nraw: %s", err, resp)
	}
	toolNames := make([]string, 0, len(list.Result.Tools))
	for _, tl := range list.Result.Tools {
		toolNames = append(toolNames, tl.Name)
	}
	t.Logf("listed %d tools: %v", len(toolNames), toolNames)

	wantTools := []string{
		"create_review", "get_review", "list_reviews", "cancel_review",
		"rerun_review", "get_review_findings", "get_suggestion_patch",
		"preview_change_files", "scan_repository",
		"list_provider_repos", "list_pull_requests", "get_branch_sha", "get_pull_request",
	}
	for _, want := range wantTools {
		if !contains(toolNames, want) {
			t.Errorf("tool %q not found in listTools", want)
		}
	}

	// 4. create_review 缺 head_sha → invalid_param
	resp = sendJSONRPC(t, stdin, stdout, 4, "tools/call", map[string]any{
		"name": "create_review",
		"arguments": map[string]any{
			"provider": "github",
			"repo":     "foo/bar",
			// 故意缺 head_sha
		},
	})
	if cat := assertToolError(t, resp, ErrCategoryInvalidParam); cat == "" {
		t.Errorf("expected invalid_param error for missing head_sha, got: %s", resp)
	} else {
		t.Logf("create_review(missing head_sha) returned category=%s as expected", cat)
	}

	// 5. create_review provider=foobar (DB 里没配) → unsupported_provider
	// 必须传 base_sha 一起，否则 service.Validate 会先报 base_sha required，
	// 拿不到 provider 检查。
	resp = sendJSONRPC(t, stdin, stdout, 5, "tools/call", map[string]any{
		"name": "create_review",
		"arguments": map[string]any{
			"provider": "foobar",
			"repo":     "foo/bar",
			"head_sha": "abc1234",
			"base_sha": "abc1233",
		},
	})
	if cat := assertToolError(t, resp, ErrCategoryUnsupported); cat == "" {
		t.Errorf("expected unsupported_provider for unknown provider, got: %s", resp)
	} else {
		t.Logf("create_review(unknown provider) returned category=%s as expected", cat)
	}
}

// sendJSONRPC 写一个 JSON-RPC 消息到 stdin 并从 stdout 读一行响应。
//
// 简化版：MCP over stdio 用 newline-delimited JSON，server 写入「响应 + \n」。
// 真实场景下 server 可能发多条 notification（progress），所以严格说要用 line-based
// 多路解析；但 e2e 测试里 server 还没启动业务逻辑，不会发 notification，简化即可。
func sendJSONRPC(t *testing.T, stdin io.Writer, stdout io.Reader, id int, method string, params any) []byte {
	t.Helper()
	msg := map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"method":  method,
	}
	if params != nil {
		msg["params"] = params
	}
	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	data = append(data, '\n')
	if _, err := stdin.Write(data); err != nil {
		t.Fatalf("stdin write: %v", err)
	}

	type result struct {
		line []byte
		err  error
	}
	ch := make(chan result, 1)
	go func() {
		br := bufio.NewReader(stdout)
		line, err := br.ReadBytes('\n')
		ch <- result{line: line, err: err}
	}()
	select {
	case r := <-ch:
		if r.err != nil {
			t.Fatalf("read response: %v", r.err)
		}
		return r.line
	case <-time.After(10 * time.Second):
		t.Fatalf("timeout waiting for response to %s (id=%d)", method, id)
		return nil
	}
}

// assertToolError 解析 CallToolResult，期望 isError=true 且 StructuredContent.category=expect。
// 返回实际 category（用于 t.Logf）。不匹配时返回 "" 让 caller 决定 fail 信息。
func assertToolError(t *testing.T, resp []byte, expect string) string {
	t.Helper()
	var call struct {
		Result struct {
			IsError           bool           `json:"isError"`
			StructuredContent map[string]any `json:"structuredContent"`
		} `json:"result"`
	}
	if err := json.Unmarshal(resp, &call); err != nil {
		t.Logf("unmarshal failed: %v\nraw: %s", err, resp)
		return ""
	}
	if !call.Result.IsError {
		return ""
	}
	cat, _ := call.Result.StructuredContent["category"].(string)
	if cat != expect {
		return ""
	}
	return cat
}

// buildMCPServerBinary 编译 cmd/mcp-server 到 /tmp，返回路径。
func buildMCPServerBinary(t *testing.T) string {
	t.Helper()
	root := repoRoot(t)
	bin := filepath.Join(os.TempDir(), "mcp-server-e2e")
	cmd := exec.Command("go", "build", "-o", bin, "./cmd/mcp-server")
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("build mcp-server (cwd=%s): %v\n%s", root, err, out)
	}
	return bin
}

// repoRoot 找 git 根目录（测试可能在 internal/mcp/ 下跑）。
func repoRoot(t *testing.T) string {
	t.Helper()
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git rev-parse: %v", err)
	}
	return strings.TrimSpace(string(out))
}

// dbConnString 拿 testcontainers 暴露给 host 的连接串。
func dbConnString(t *testing.T, pg testcontainers.Container, ctx context.Context) string {
	t.Helper()
	host, err := pg.Host(ctx)
	if err != nil {
		t.Fatalf("postgres host: %v", err)
	}
	port, err := pg.MappedPort(ctx, "5432/tcp")
	if err != nil {
		t.Fatalf("postgres port: %v", err)
	}
	return fmt.Sprintf("postgres://postgres:postgres@%s:%s/review_test?sslmode=disable", host, port.Port())
}

func contains(s []string, want string) bool {
	for _, v := range s {
		if v == want {
			return true
		}
	}
	return false
}

// isDockerUnavailable 粗略判断 testcontainers 报错是因为 docker 不可用（CI 没 daemon）。
// 避免对网络抖动 / 拉镜像超时等其它错误误判。
func isDockerUnavailable(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "Cannot connect to the Docker daemon") ||
		strings.Contains(msg, "docker.sock") ||
		strings.Contains(msg, "open //./pipe/docker_engine")
}

// TestE2ECompileOnly 极简的「能跑就行」 sanity check：测 fx wire 路径不会出现编译期循环。
func TestE2ECompileOnly(t *testing.T) {
	var _ app.ReviewService = (app.ReviewService)(nil)
	_ = context.Background
	_ = fmt.Sprintf
}
