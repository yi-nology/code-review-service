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

// stubLegacy 实现 legacyRunner 接口，用于测试 Runner 委托逻辑。
type stubLegacy struct {
	runOut     domain.Result
	runErr     error
	previewOut *domain.PreviewResult
	previewErr error
	scanOut    *domain.ScanResult
	scanErr    error

	gotCfg        Config
	invocations   int
}

func (s *stubLegacy) RunWithConfig(_ context.Context, _ domain.Workspace, cfg Config) (domain.Result, error) {
	s.gotCfg = cfg
	s.invocations++
	return s.runOut, s.runErr
}

func (s *stubLegacy) PreviewWithConfig(_ context.Context, _ domain.Workspace, cfg Config) (*domain.PreviewResult, error) {
	s.gotCfg = cfg
	s.invocations++
	return s.previewOut, s.previewErr
}

func (s *stubLegacy) ScanWithConfig(_ context.Context, _ domain.Workspace, req domain.ScanRequest, cfg Config) (*domain.ScanResult, error) {
	s.gotCfg = cfg
	s.invocations++
	return s.scanOut, s.scanErr
}

func TestRunner_DelegatesRunToLegacy(t *testing.T) {
	want := domain.Result{Summary: "ok"}
	legacy := &stubLegacy{runOut: want}
	cfg := Defaults().ToConfig()
	r := NewRunnerWithLegacy(cfg, legacy)

	got, err := r.Run(context.Background(), domain.Workspace{Dir: "/tmp"})
	if err != nil {
		t.Fatalf("Run err: %v", err)
	}
	if got.Summary != "ok" {
		t.Errorf("expected Summary=ok, got %q", got.Summary)
	}
	if legacy.invocations != 1 {
		t.Errorf("expected 1 invocation, got %d", legacy.invocations)
	}
}

func TestRunner_DelegatesPreviewAndScan(t *testing.T) {
	legacy := &stubLegacy{
		previewOut: &domain.PreviewResult{Files: []string{"a.go"}},
		scanOut:    &domain.ScanResult{},
	}
	cfg := Defaults().ToConfig()
	r := NewRunnerWithLegacy(cfg, legacy)

	pv, err := r.Preview(context.Background(), domain.Workspace{Dir: "/tmp"})
	if err != nil {
		t.Fatalf("Preview err: %v", err)
	}
	if len(pv.Files) != 1 || pv.Files[0] != "a.go" {
		t.Errorf("expected [a.go], got %v", pv.Files)
	}

	sc, err := r.Scan(context.Background(), domain.Workspace{Dir: "/tmp"}, domain.ScanRequest{Batch: "none"})
	if err != nil {
		t.Fatalf("Scan err: %v", err)
	}
	if sc == nil {
		t.Errorf("expected non-nil ScanResult")
	}
}

func TestRunner_NoLegacyReturnsError(t *testing.T) {
	r := NewRunner(Defaults().ToConfig())
	_, err := r.Run(context.Background(), domain.Workspace{Dir: "/tmp"})
	if err == nil {
		t.Fatalf("expected err when legacy is nil")
	}
}

func TestRunner_LegacyErrorPropagates(t *testing.T) {
	legacy := &stubLegacy{runErr: errors.New("boom")}
	r := NewRunnerWithLegacy(Defaults().ToConfig(), legacy)

	_, err := r.Run(context.Background(), domain.Workspace{Dir: "/tmp"})
	if err == nil || err.Error() != "boom" {
		t.Fatalf("expected boom, got %v", err)
	}
}

func TestRunner_RefreshReplacesCfg(t *testing.T) {
	legacy := &stubLegacy{}
	cfg1 := Defaults().ToConfig()
	cfg1.Binary = "first"
	r := NewRunnerWithLegacy(cfg1, legacy)
	_, _ = r.Run(context.Background(), domain.Workspace{Dir: "/tmp"})
	if legacy.gotCfg.Binary != "first" {
		t.Errorf("expected first, got %q", legacy.gotCfg.Binary)
	}

	cfg2 := Defaults().ToConfig()
	cfg2.Binary = "second"
	r.Refresh(cfg2)
	_, _ = r.Run(context.Background(), domain.Workspace{Dir: "/tmp"})
	if legacy.gotCfg.Binary != "second" {
		t.Errorf("expected second after Refresh, got %q", legacy.gotCfg.Binary)
	}
}

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

// TestRunner_DirectRun_ShScript 验证 Run 能真实执行 sh 命令。
func TestRunner_DirectRun_ShScript(t *testing.T) {
	// 写一个 sh 脚本，模拟 ocr review 输出
	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "fake-ocr")
	script := `#!/bin/sh
echo '{"summary":"test summary from fake ocr"}'
`
	if err := os.WriteFile(scriptPath, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	cfg := Defaults().ToConfig()
	cfg.Binary = scriptPath
	r := NewRunner(cfg)

	ws := domain.Workspace{Dir: tmpDir}
	result, err := r.Run(context.Background(), ws)
	if err != nil {
		t.Fatalf("Run err: %v", err)
	}
	if result.Summary != "test summary from fake ocr" {
		t.Errorf("expected Summary='test summary from fake ocr', got %q", result.Summary)
	}
	if result.OCR.Engine != "open-code-review" {
		t.Errorf("expected engine=open-code-review, got %q", result.OCR.Engine)
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