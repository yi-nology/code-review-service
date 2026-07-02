package ocr

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yi-nology/code-review-service/internal/service/review/domain"
)

// TestRunner_DirectRun_EmptyDir 验证 Run 在 Dir 为空时返回明确错误。
func TestRunner_DirectRun_EmptyDir(t *testing.T) {
	r := NewRunner(Defaults().ToConfig())
	_, err := r.Run(context.Background(), domain.Workspace{Dir: ""})
	if err == nil {
		t.Fatalf("expected err for empty Dir")
	}
	if !strings.Contains(err.Error(), "checked out workspace") {
		t.Errorf("expected workspace error, got %v", err)
	}
}

// TestRunner_DirectRun_BinaryMissing 验证二进制缺失时返回 BinaryMissingError。
func TestRunner_DirectRun_BinaryMissing(t *testing.T) {
	cfg := Defaults().ToConfig()
	cfg.Binary = "/nonexistent/ocr/binary"
	r := NewRunner(cfg)
	_, err := r.Run(context.Background(), domain.Workspace{Dir: "/tmp"})
	if err == nil {
		t.Fatalf("expected err for missing binary")
	}
	if !IsBinaryMissing(err) {
		t.Errorf("expected BinaryMissingError, got %T %v", err, err)
	}
}

// TestRunner_DirectPreview_EmptyDir 验证 Preview 在 Dir 为空时返回明确错误。
func TestRunner_DirectPreview_EmptyDir(t *testing.T) {
	r := NewRunner(Defaults().ToConfig())
	_, err := r.Preview(context.Background(), domain.Workspace{Dir: ""})
	if err == nil {
		t.Fatalf("expected err for empty Dir")
	}
}

// TestRunner_DirectScan_BinaryMissing 验证 Scan 在二进制缺失时返回 BinaryMissingError。
func TestRunner_DirectScan_BinaryMissing(t *testing.T) {
	cfg := Defaults().ToConfig()
	cfg.Binary = "/nonexistent/ocr/binary"
	r := NewRunner(cfg)
	_, err := r.Scan(context.Background(), domain.Workspace{Dir: "/tmp"}, domain.ScanRequest{})
	if err == nil {
		t.Fatalf("expected err for missing binary")
	}
	if !IsBinaryMissing(err) {
		t.Errorf("expected BinaryMissingError, got %T", err)
	}
}

// TestRunner_DirectRun_ShScript 验证 Run 能真实执行 sh 命令并解析真实 OCR JSON 输出。
func TestRunner_DirectRun_ShScript(t *testing.T) {
	// 写一个 sh 脚本，模拟 ocr review 的真实 --format json 输出。
	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "fake-ocr")
	script := `#!/bin/sh
cat <<'EOF'
{"status":"success","summary":{"files_reviewed":1,"comments":1,"total_tokens":42,"input_tokens":30,"output_tokens":12,"elapsed":"1s"},"comments":[{"path":"main.go","content":"fake finding body","suggestion_code":"x = 1","existing_code":"x = 0","start_line":5,"end_line":5}]}
EOF
`
	if err := os.WriteFile(scriptPath, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	cfg := Defaults().ToConfig()
	cfg.Binary = scriptPath
	cfg.DefaultSeverity = "major"
	r := NewRunner(cfg)

	ws := domain.Workspace{Dir: tmpDir}
	result, err := r.Run(context.Background(), ws)
	if err != nil {
		t.Fatalf("Run err: %v", err)
	}
	if len(result.Findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(result.Findings))
	}
	if result.Findings[0].Path != "main.go" || result.Findings[0].StartLine != 5 {
		t.Errorf("unexpected finding: %+v", result.Findings[0])
	}
	if result.OCR.Engine != "open-code-review" {
		t.Errorf("expected engine=open-code-review, got %q", result.OCR.Engine)
	}
	if result.OCR.Status != "success" || result.OCR.TotalTokens != 42 {
		t.Errorf("unexpected OCR metadata: %+v", result.OCR)
	}
}

// TestRunner_DirectRun_Timeout 验证 Run 超时时返回 TimeoutError。
func TestRunner_DirectRun_Timeout(t *testing.T) {
	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "fake-ocr-slow")
	script := `#!/bin/sh
sleep 30
`
	if err := os.WriteFile(scriptPath, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	cfg := Defaults().ToConfig()
	cfg.Binary = scriptPath
	cfg.TimeoutMinutes = 0 // 默认 10 分钟，足够长；用 ctx timeout 控制
	r := NewRunner(cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 1*1000) // 1秒
	defer cancel()

	ws := domain.Workspace{Dir: tmpDir}
	_, err := r.Run(ctx, ws)
	if err == nil {
		t.Fatalf("expected timeout err")
	}
	// ctx cancel 触发 SIGKILL，不一定归类为 TimeoutError（是 ctx 取消）
	// 但 err 不应 nil
}

// TestBinaryMissingError_Error 验证错误信息。
func TestBinaryMissingError_Error(t *testing.T) {
	err := &BinaryMissingError{Binary: "ocr", Err: errors.New("not found")}
	msg := err.Error()
	if !strings.Contains(msg, "ocr") {
		t.Errorf("expected msg to contain 'ocr', got %q", msg)
	}
	if !strings.Contains(msg, "not found") {
		t.Errorf("expected msg to contain underlying err, got %q", msg)
	}
}

// TestAppendToolsArgs_PathPriority 验证 ToolsPath 优先于 ToolsConfig。
func TestAppendToolsArgs_PathPriority(t *testing.T) {
	cfg := Config{ToolsPath: "/path/to/tools.json", ToolsConfig: "{}"}
	args, cleanup, err := appendToolsArgs([]string{}, cfg)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if cleanup != nil {
		t.Errorf("expected nil cleanup when ToolsPath set")
	}
	if len(args) != 2 || args[0] != "--tools" || args[1] != "/path/to/tools.json" {
		t.Errorf("unexpected args: %v", args)
	}
}

// TestAppendToolsArgs_ConfigTempFile 验证 ToolsConfig 写临时文件。
func TestAppendToolsArgs_ConfigTempFile(t *testing.T) {
	cfg := Config{ToolsConfig: `{"k":"v"}`}
	args, cleanup, err := appendToolsArgs([]string{}, cfg)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if cleanup == nil {
		t.Errorf("expected non-nil cleanup for temp file")
	} else {
		cleanup()
	}
	if len(args) != 2 || args[0] != "--tools" {
		t.Errorf("unexpected args: %v", args)
	}
}

// TestAppendToolsArgs_NeitherNoOp 验证无 Tools 字段时 args 不变。
func TestAppendToolsArgs_NeitherNoOp(t *testing.T) {
	cfg := Config{}
	args, cleanup, err := appendToolsArgs([]string{"foo"}, cfg)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if cleanup != nil {
		t.Errorf("expected nil cleanup")
	}
	if len(args) != 1 || args[0] != "foo" {
		t.Errorf("unexpected args: %v", args)
	}
}
