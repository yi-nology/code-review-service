package ocr

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yi-nology/code-review-service/internal/service/review/domain"
)

// TestParseDelegatePreview 校验 `ocr delegate preview` 的 markdown 输出被正确解析。
//
// 真实输出形如：
//
//	# Files (1 reviewable / 1 total)
//	- mode: range
//	- from: HEAD~1
//	- to: HEAD
//	- merge_base: <sha>
//	- total_insertions: 2
//	- total_deletions: 0
//	  - `main.go` [modified] +2/-0
func TestParseDelegatePreview(t *testing.T) {
	raw := `# Files (2 reviewable / 3 total)

- mode: range
- from: HEAD~1
- to: HEAD
- merge_base: abc123def456
- total_insertions: 10
- total_deletions: 3

  - ` + "`main.go`" + ` [modified] +8/-1
  - ` + "`internal/agent/agent.go`" + ` [added] +2/-0
`
	got := parseDelegatePreview(raw, "HEAD~1", "HEAD")
	if got.Mode != "range" {
		t.Errorf("Mode = %q, want range", got.Mode)
	}
	if got.From != "HEAD~1" || got.To != "HEAD" {
		t.Errorf("From/To = %q/%q, want HEAD~1/HEAD", got.From, got.To)
	}
	if got.MergeBase != "abc123def456" {
		t.Errorf("MergeBase = %q, want abc123def456", got.MergeBase)
	}
	if got.TotalInsert != 10 || got.TotalDelete != 3 {
		t.Errorf("Insert/Delete = %d/%d, want 10/3", got.TotalInsert, got.TotalDelete)
	}
	wantFiles := []string{"main.go", "internal/agent/agent.go"}
	if len(got.Files) != len(wantFiles) {
		t.Fatalf("Files = %v, want %v", got.Files, wantFiles)
	}
	for i, w := range wantFiles {
		if got.Files[i] != w {
			t.Errorf("Files[%d] = %q, want %q", i, got.Files[i], w)
		}
	}
	if got.RawOutput != raw {
		t.Errorf("RawOutput should preserve original input")
	}
}

// TestParseDelegatePreview_WorkspaceMode 校验无 from/to 的 workspace 模式。
func TestParseDelegatePreview_WorkspaceMode(t *testing.T) {
	raw := `# Files (0 reviewable / 0 total)

- mode: workspace
`
	got := parseDelegatePreview(raw, "", "")
	if got.Mode != "workspace" {
		t.Errorf("Mode = %q, want workspace", got.Mode)
	}
	if len(got.Files) != 0 {
		t.Errorf("expected 0 files, got %v", got.Files)
	}
}

// TestParseDelegatePreview_MalformedLinesDontPanic 校验格式异常的行不引发 panic，已解析的部分保留。
func TestParseDelegatePreview_MalformedLinesDontPanic(t *testing.T) {
	raw := `garbage line
- mode: range
- broken: no colon should be skipped
- total_insertions: not-a-number
  - not a file line
  - ` + "`valid.go`" + `
`
	got := parseDelegatePreview(raw, "", "")
	if got.Mode != "range" {
		t.Errorf("Mode = %q, want range", got.Mode)
	}
	// not-a-number 应被 strconv.Atoi 安全吞掉为 0。
	if got.TotalInsert != 0 {
		t.Errorf("TotalInsert = %d, want 0 (malformed)", got.TotalInsert)
	}
	if len(got.Files) != 1 || got.Files[0] != "valid.go" {
		t.Errorf("Files = %v, want [valid.go]", got.Files)
	}
}

// TestRunner_DelegatePreview_PassesArgs 用 fake sh binary 验证 delegate preview 的 argv 构造。
//
// 这是 e2e 等价的单元测试：不依赖真实 ocr binary，只断言 args 正确（--repo/--from/--to/--exclude）。
func TestRunner_DelegatePreview_PassesArgs(t *testing.T) {
	tmpDir := t.TempDir()
	argLogFile := filepath.Join(tmpDir, "argv")
	scriptPath := filepath.Join(tmpDir, "fake-ocr")
	// 模拟 delegate preview 输出 + 记录 argv。
	script := "#!/bin/sh\n" +
		fmt.Sprintf("printf '%%s\\n' \"$@\" > %q\n", argLogFile) +
		"cat <<'EOF'\n" +
		"# Files (1 reviewable / 1 total)\n" +
		"- mode: range\n" +
		"- from: base123\n" +
		"- to: head456\n" +
		"- merge_base: mb789\n" +
		"- total_insertions: 5\n" +
		"- total_deletions: 1\n" +
		"  - `main.go` [modified] +5/-1\n" +
		"EOF\n"
	if err := os.WriteFile(scriptPath, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	cfg := Defaults().ToConfig()
	cfg.Binary = scriptPath
	r := NewRunner(cfg)

	ws := domain.Workspace{
		Dir:         tmpDir,
		BaseSHA:     "base123",
		HeadSHA:     "head456",
		ScanExclude: []string{"vendor/**", "dist/**"},
	}
	got, err := r.DelegatePreview(context.Background(), ws)
	if err != nil {
		t.Fatalf("DelegatePreview err: %v", err)
	}
	if got.Mode != "range" || got.MergeBase != "mb789" {
		t.Errorf("parsed result unexpected: %+v", got)
	}
	if len(got.Files) != 1 || got.Files[0] != "main.go" {
		t.Errorf("Files = %v, want [main.go]", got.Files)
	}

	argv, _ := os.ReadFile(argLogFile)
	argvStr := string(argv)
	for _, want := range []string{"delegate", "preview", "--repo", "--from", "base123", "--to", "head456", "--exclude", "vendor/**,dist/**"} {
		if !strings.Contains(argvStr, want) {
			t.Errorf("expected %q in OCR args, got:\n%s", want, argvStr)
		}
	}
}

// TestRunner_DelegateRule_PassesArgs 校验 delegate rule 把 paths 作为位置参数传递。
func TestRunner_DelegateRule_PassesArgs(t *testing.T) {
	tmpDir := t.TempDir()
	argLogFile := filepath.Join(tmpDir, "argv")
	scriptPath := filepath.Join(tmpDir, "fake-ocr")
	script := "#!/bin/sh\n" +
		fmt.Sprintf("printf '%%s\\n' \"$@\" > %q\n", argLogFile) +
		"echo '### Rule Group 1: system / default'\n"
	if err := os.WriteFile(scriptPath, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	cfg := Defaults().ToConfig()
	cfg.Binary = scriptPath
	r := NewRunner(cfg)

	ws := domain.Workspace{Dir: tmpDir}
	paths := []string{"a.go", "b.go"}
	got, err := r.DelegateRule(context.Background(), ws, paths)
	if err != nil {
		t.Fatalf("DelegateRule err: %v", err)
	}
	if !strings.Contains(got.RawOutput, "Rule Group 1") {
		t.Errorf("RawOutput missing rule group, got: %q", got.RawOutput)
	}

	argv, _ := os.ReadFile(argLogFile)
	argvStr := string(argv)
	// 位置参数（paths）应在 --rule 等选项之后。
	for _, want := range []string{"delegate", "rule", "--repo", "a.go", "b.go"} {
		if !strings.Contains(argvStr, want) {
			t.Errorf("expected %q in OCR args, got:\n%s", want, argvStr)
		}
	}
}

// TestRunner_DelegatePreview_EmptyDir 校验无工作区时返回明确错误。
func TestRunner_DelegatePreview_EmptyDir(t *testing.T) {
	r := NewRunner(Defaults().ToConfig())
	if _, err := r.DelegatePreview(context.Background(), domain.Workspace{Dir: ""}); err == nil {
		t.Errorf("expected error for empty Dir")
	}
	if _, err := r.DelegateRule(context.Background(), domain.Workspace{Dir: ""}, nil); err == nil {
		t.Errorf("expected error for empty Dir")
	}
}
