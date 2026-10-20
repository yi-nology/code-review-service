package mcp

// e2e_helpers_test.go 抽出 mcp e2e 测试共享 helper，给 e2e_sse_test.go 复用。
// 原来这些 helper 散落在 e2e_stdio_test.go 里，stdio 模式砍掉后迁移到本文件。

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yi-nology/code-review-service/internal/app"

	"github.com/testcontainers/testcontainers-go"
)

// dockerSkip 是 docker 不可用时统一 skip 原因，e2e_sse_test 共用。
var dockerSkip = errors.New("testcontainers: docker unavailable; skipping mcp e2e")

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

// assertToolError 解析 CallToolResult，期望 isError=true 且 StructuredContent.category=expect。
// 返回实际 category（用于 t.Logf）。不匹配时返回 "" 让 caller 决定 fail 信息。
//
// mcp-go 的 JSON-RPC 响应（HTTP / streamable_http / stdio 都一样）：
//
//	{"jsonrpc":"2.0","id":N,"result":{"isError":true,"structuredContent":{"category":"..."}}}
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

// TestE2ECompileOnly 极简的「能跑就行」 sanity check：测 fx wire 路径不会出现编译期循环。
func TestE2ECompileOnly(t *testing.T) {
	var _ app.ReviewService = (app.ReviewService)(nil)
	_ = context.Background
	_ = fmt.Sprintf
}
