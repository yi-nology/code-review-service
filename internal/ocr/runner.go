package ocr

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/yi-nology/code-review-service/internal/service/progress"
	"github.com/yi-nology/code-review-service/internal/service/review/domain"

	"github.com/oekit/concurrency/async"
	"github.com/oekit/logx"
	"github.com/oekit/pkg/convert"
)

// Runner 是 open-code-review 子进程调用入口（阶段 A12 完整化）。
//
// 实现了 domain.Runner / domain.PreviewRunner / domain.ScanRunner 三接口。
//
// 行为：
//   - Run: 调 "ocr review --repo --from --to ..." 子进程，解析 JSON 输出
//   - Preview: 调 "ocr scan --preview ..." 子进程
//   - Scan: 调 "ocr scan ..." 子进程，支持 batch / paths / exclude
//
// 阶段 A12：完整吸收 runner/runner.go + runner/preview.go + runner/scan.go 业务逻辑。
type Runner struct {
	cfg Config
}

// Compile-time assertions
var (
	_ domain.Runner        = (*Runner)(nil)
	_ domain.PreviewRunner = (*Runner)(nil)
	_ domain.ScanRunner    = (*Runner)(nil)
)

// NewRunner 从 Config 构造 Runner。
func NewRunner(cfg Config) *Runner {
	return &Runner{cfg: cfg}
}

// Cfg 返回当前 Config。
func (r *Runner) Cfg() Config { return r.cfg }

// Refresh 替换 Config。
func (r *Runner) Refresh(cfg Config) { r.cfg = cfg }

// Previewer / Scanner 包装 Runner。
func (r *Runner) Previewer() domain.PreviewRunner { return r }
func (r *Runner) Scanner() domain.ScanRunner { return r }

// defaultOCRMaxRetries 默认不重试。
const defaultOCRMaxRetries = 0

// Run 实现 domain.Runner。
//
// 阶段 A12 完整化：直接调 ocr 二进制，保留原 runner.OpenCodeReviewRunner 的所有行为：
//   - 超时 + 重试（默认 0 次，配置可调）
//   - ctx cancel → SIGKILL 三段式
//   - stderr 行级回调（用于 progress 解析）
//   - JSON 输出解析
func (r *Runner) Run(ctx context.Context, workspace domain.Workspace) (domain.Result, error) {
	if strings.TrimSpace(workspace.Dir) == "" {
		return domain.Result{}, fmt.Errorf("open-code-review requires a checked out workspace")
	}

	timeout := time.Duration(r.cfg.TimeoutMinutes) * time.Minute
	if timeout <= 0 {
		timeout = 10 * time.Minute
	}

	maxRetries := r.cfg.MaxRetries
	if maxRetries <= 0 {
		maxRetries = defaultOCRMaxRetries
	}
	attempts := maxRetries + 1

	reporter := progress.ProgressReporterFromContext(ctx)

	var lastErr error
	for attempt := 1; attempt <= attempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return domain.Result{}, err
		}

		attemptCtx, cancel := context.WithTimeout(ctx, timeout+30*time.Second)
		if reporter != nil {
			attemptCtx = progress.WithProgressReporter(attemptCtx, reporter)
		}

		if attempt > 1 {
			logx.CtxWarnf(ctx, "ocr run failed, retrying attempt %d/%d: %v", attempt-1, maxRetries, lastErr)
			if reporter != nil {
				reporter.ReportAgentStatus("ocr", "retrying", fmt.Sprintf("重试第 %d 次", attempt-1))
			}
		}

		result, err := r.runOnce(attemptCtx, workspace, timeout)
		cancel()
		if err == nil {
			return result, nil
		}
		lastErr = err

		if ctx.Err() != nil {
			break
		}
	}
	return domain.Result{}, lastErr
}

func (r *Runner) runOnce(ctx context.Context, workspace domain.Workspace, timeout time.Duration) (domain.Result, error) {
	fromRef := workspace.BaseSHA
	toRef := workspace.HeadSHA
	if fromRef == "" {
		fromRef = "HEAD"
	}

	if lang := strings.TrimSpace(r.cfg.Language); lang != "" {
		ensureOCRLanguage(r.cfg.Binary, lang)
	}

	audience := r.cfg.Audience
	if audience == "" {
		audience = "human"
	}

	args := []string{
		"review",
		"--repo", workspace.Dir,
		"--from", fromRef,
		"--to", toRef,
		"--format", "json",
		"--concurrency", convert.IntToStr(r.cfg.Concurrency),
		"--max-git-procs", convert.IntToStr(r.cfg.MaxGitProcs),
		"--timeout", convert.IntToStr(r.cfg.TimeoutMinutes),
		"--audience", audience,
	}
	rulePath := r.cfg.RulePath
	if strings.TrimSpace(workspace.RulePath) != "" {
		rulePath = workspace.RulePath
	}
	if strings.TrimSpace(rulePath) != "" {
		args = append(args, "--rule", rulePath)
	}
	if r.cfg.MaxTools > 0 {
		args = append(args, "--max-tools", convert.IntToStr(r.cfg.MaxTools))
	}
	toolsArgs, cleanupTools, err := appendToolsArgs(args, r.cfg)
	if err != nil {
		return domain.Result{}, err
	}
	args = toolsArgs
	if cleanupTools != nil {
		defer cleanupTools()
	}
	if strings.TrimSpace(r.cfg.Background) != "" {
		args = append(args, "--background", r.cfg.Background)
	}

	cmd := exec.CommandContext(ctx, r.cfg.Binary, args...)
	cmd.Dir = workspace.Dir
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	env := os.Environ()
	env = r.cfg.AppendEnv(env)
	cmd.Env = env

	stdout := newCappedBuffer(maxSubprocessOutput)
	cmd.Stdout = stdout

	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return domain.Result{}, fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	progressReporter := progress.ProgressReporterFromContext(ctx)

	if err := cmd.Start(); err != nil {
		return domain.Result{}, &BinaryMissingError{Binary: r.cfg.Binary, Err: err}
	}

	var stderrLines []string
	var stderrMu sync.Mutex
	var stderrWg sync.WaitGroup
	stderrWg.Add(1)
	async.GoSafe(func() {
		defer stderrWg.Done()
		scanner := bufio.NewScanner(stderrPipe)
		scanner.Buffer(make([]byte, 0, 64*1024), 1<<20)
		for scanner.Scan() {
			line := scanner.Text()
			stderrMu.Lock()
			stderrLines = append(stderrLines, line)
			stderrMu.Unlock()

			if progressReporter != nil {
				parseOCRProgress(line, progressReporter)
			}
		}
		if err := scanner.Err(); err != nil {
			logx.Warnf("ocr stderr scanner error: %v", err)
		}
	})

	done := make(chan struct{})
	async.GoSafe(func() {
		select {
		case <-ctx.Done():
			if cmd.Process != nil {
				_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
			}
		case <-done:
		}
	})

	err = cmd.Wait()
	close(done)
	stderrWg.Wait()
	if err != nil {
		if cmd.Process != nil {
			_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		}
		stderrMu.Lock()
		message := strings.Join(stderrLines, "\n")
		stderrMu.Unlock()
		return domain.Result{}, classifyExitError(ctx, err, timeout, message)
	}

	return parseOpenCodeReviewOutput(stdout.Bytes(), r.cfg.DefaultSeverity), nil
}


// ---- 工具函数（从 runner 包迁移） ----

// BinaryMissingError 表示 OCR 二进制不存在或不可执行（cmd.Start 失败）。
type BinaryMissingError struct {
	Binary string
	Err    error
}

func (e *BinaryMissingError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("ocr 二进制 %q 不可用: %v", e.Binary, e.Err)
	}
	return fmt.Sprintf("ocr 二进制 %q 不可用", e.Binary)
}

func (e *BinaryMissingError) Unwrap() error { return e.Err }

// appendToolsArgs 把 tools_path 或 tools_config 加到 args。
func appendToolsArgs(args []string, cfg Config) ([]string, func(), error) {
	if strings.TrimSpace(cfg.ToolsPath) != "" {
		args = append(args, "--tools", cfg.ToolsPath)
		return args, nil, nil
	}
	if strings.TrimSpace(cfg.ToolsConfig) != "" {
		f, err := os.CreateTemp("", "ocr-tools-*.json")
		if err != nil {
			return args, nil, fmt.Errorf("create tools config temp file: %w", err)
		}
		if _, err := f.WriteString(cfg.ToolsConfig); err != nil {
			_ = f.Close()
			_ = os.Remove(f.Name())
			return args, nil, fmt.Errorf("write tools config: %w", err)
		}
		_ = f.Close()
		args = append(args, "--tools", f.Name())
		return args, func() { _ = os.Remove(f.Name()) }, nil
	}
	return args, nil, nil
}

// parseOCRProgress 从 stderr 一行文本中解析 OCR 进度并通过 reporter 上报。
// OCR CLI 输出形如 "progress: 50% phase=context"。
// 此前该函数是空桩（_ = reporter），导致 WS 进度 UI 拿不到 OCR 子步骤进度。
func parseOCRProgress(line string, reporter progress.ProgressReporter) {
	const prefix = "progress:"
	if !strings.HasPrefix(line, prefix) {
		return
	}
	rest := strings.TrimSpace(line[len(prefix):])
	fields := strings.Fields(rest)

	var pct float64
	var phaseName string
	for _, f := range fields {
		switch {
		case strings.HasSuffix(f, "%"):
			if v, err := strconv.ParseFloat(strings.TrimSuffix(f, "%"), 64); err == nil {
				pct = v
			}
		case strings.HasPrefix(f, "phase="):
			phaseName = strings.TrimPrefix(f, "phase=")
		}
	}

	// 归一化百分比，防止异常输入。
	if pct < 0 {
		pct = 0
	} else if pct > 100 {
		pct = 100
	}

	// 已知 phase 名与 progress.Phase 常量一致，直接映射以驱动前端阶段切换。
	if _, ok := knownOCRPhases[phaseName]; ok {
		reporter.UpdatePhase(progress.Phase(phaseName), "active", pct, "ocr: "+rest)
		return
	}
	reporter.ReportProgress("ocr: "+rest, pct)
}

// knownOCRPhases 列出 progress.Phase 体系支持的阶段名（见 progress/types.go）。
var knownOCRPhases = map[string]struct{}{
	"context":        {},
	"change_context": {},
	"analysis":       {},
	"reviews":        {},
	"aggregation":    {},
	"discourse":      {},
	"synthesis":      {},
	"complete":       {},
}

// maxSubprocessOutput 限制单个 OCR 子进程 stdout/stderr 的内存上限，
// 防止健谈/失控的二进制把内存撑爆（OOM）。正常 JSON 输出远小于此。
const maxSubprocessOutput = 64 << 20 // 64 MB

// cappedBuffer 是带容量上限的 bytes.Buffer：超过 cap 后丢弃后续写入并记录一次告警，
// 作为 exec.Cmd 的 Stdout/Stdout 捕获器，限制输出内存占用。
type cappedBuffer struct {
	buf    bytes.Buffer
	cap    int
	capped bool
}

func newCappedBuffer(capBytes int) *cappedBuffer {
	return &cappedBuffer{cap: capBytes}
}

func (b *cappedBuffer) Write(p []byte) (int, error) {
	if b.capped {
		return len(p), nil
	}
	room := b.cap - b.buf.Len()
	if len(p) >= room {
		if room > 0 {
			b.buf.Write(p[:room])
		}
		b.capped = true
		logx.Warnf("ocr subprocess output exceeded %d bytes cap, further output discarded", b.cap)
		return len(p), nil
	}
	return b.buf.Write(p)
}

func (b *cappedBuffer) Bytes() []byte  { return b.buf.Bytes() }
func (b *cappedBuffer) String() string { return b.buf.String() }

// classifyExitError 把 cmd.Wait 的失败归类为 TimeoutError 或 ExitError。
func classifyExitError(ctx context.Context, runErr error, timeout time.Duration, stderr string) error {
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return &TimeoutError{Timeout: timeout}
	}
	return &ExitError{Stderr: stderr, Err: runErr}
}

// TimeoutError 表示 OCR 评审超过 context deadline。
type TimeoutError struct {
	Timeout time.Duration
}

func (e *TimeoutError) Error() string {
	return fmt.Sprintf("ocr 评审超时（上限 %s）", e.Timeout)
}

// ExitError 表示 OCR 子进程非零退出，Stderr 含根因。
type ExitError struct {
	Stderr string
	Err    error
}

func (e *ExitError) Error() string {
	msg := strings.TrimSpace(e.Stderr)
	if msg == "" && e.Err != nil {
		msg = e.Err.Error()
	}
	if msg == "" {
		return "ocr 评审失败"
	}
	return fmt.Sprintf("ocr 评审失败: %s", msg)
}

func (e *ExitError) Unwrap() error { return e.Err }

// IsBinaryMissing / IsTimeout / IsExit 供上层用 errors.As 判断。
func IsBinaryMissing(err error) bool {
	var e *BinaryMissingError
	return errors.As(err, &e)
}

func IsTimeout(err error) bool {
	var e *TimeoutError
	return errors.As(err, &e)
}

func IsExit(err error) bool {
	var e *ExitError
	return errors.As(err, &e)
}

// parseOpenCodeReviewOutput 解析 `ocr review --format json` 的 JSON 输出。
//
// OCR 输出结构：
//
//	{
//	  "status": "success",
//	  "summary": {
//	    "files_reviewed": 1, "comments": 3,
//	    "total_tokens": 23474, "input_tokens": 21694, "output_tokens": 1780,
//	    "elapsed": "35s"
//	  },
//	  "tool_calls": { "total": 5, "by_tool": { "code_search": 1, ... } },
//	  "comments": [
//	    { "path", "content", "suggestion_code", "existing_code", "start_line", "end_line" }
//	  ]
//	}
//
// comments 映射为 domain.Finding；summary 映射为 OCRMetadata。
// OCR 不输出 severity，统一用 defaultSeverity（上层归一化）。
func parseOpenCodeReviewOutput(data []byte, defaultSeverity string) domain.Result {
	if len(data) == 0 {
		return domain.Result{Summary: "ocr 评审未产生输出"}
	}

	var payload struct {
		Status  string `json:"status"`
		Summary struct {
			FilesReviewed int    `json:"files_reviewed"`
			Comments      int    `json:"comments"`
			TotalTokens   int64  `json:"total_tokens"`
			InputTokens   int64  `json:"input_tokens"`
			OutputTokens  int64  `json:"output_tokens"`
			Elapsed       string `json:"elapsed"`
		} `json:"summary"`
		ToolCalls *domain.ToolCallsMetadata `json:"tool_calls"`
		Comments  []struct {
			Path           string `json:"path"`
			Content        string `json:"content"`
			SuggestionCode string `json:"suggestion_code"`
			ExistingCode   string `json:"existing_code"`
			StartLine      int    `json:"start_line"`
			EndLine        int    `json:"end_line"`
		} `json:"comments"`
	}

	if err := json.Unmarshal(data, &payload); err != nil {
		// 非 JSON 输出（如纯文本错误）：保留原文当 summary，便于排障。
		return domain.Result{
			Summary: strings.TrimSpace(string(data)),
			OCR:     domain.OCRMetadata{Engine: "open-code-review", Status: "parse_error"},
		}
	}

	severity := strings.TrimSpace(defaultSeverity)
	if severity == "" {
		severity = "major"
	}

	findings := make([]domain.Finding, 0, len(payload.Comments))
	for _, c := range payload.Comments {
		line := c.StartLine
		if line <= 0 {
			line = c.EndLine
		}
		findings = append(findings, domain.Finding{
			Severity:       severity,
			Path:           c.Path,
			Line:           line,
			StartLine:      c.StartLine,
			EndLine:        c.EndLine,
			Title:          findingTitle(c.Content),
			Body:           c.Content,
			ExistingCode:   c.ExistingCode,
			SuggestionCode: c.SuggestionCode,
			Suggestion:     c.SuggestionCode,
		})
	}

	commentCount := payload.Summary.Comments
	if commentCount == 0 {
		commentCount = len(findings)
	}
	summary := fmt.Sprintf("ocr 评审完成：%d 条评论，%d 个文件", commentCount, payload.Summary.FilesReviewed)

	return domain.Result{
		Summary:  summary,
		Findings: findings,
		OCR: domain.OCRMetadata{
			Engine:        "open-code-review",
			Status:        payload.Status,
			FilesReviewed: int64(payload.Summary.FilesReviewed),
			Comments:      int64(commentCount),
			TotalTokens:   payload.Summary.TotalTokens,
			InputTokens:   payload.Summary.InputTokens,
			OutputTokens:  payload.Summary.OutputTokens,
			Elapsed:       payload.Summary.Elapsed,
			ToolCalls:     payload.ToolCalls,
		},
	}
}

// findingTitle 从 OCR comment 的完整内容中提取一个简短标题。
func findingTitle(content string) string {
	content = strings.TrimSpace(content)
	if content == "" {
		return ""
	}
	title := content
	for _, sep := range []string{"\n", "。", "；", "：", ":", ".", ";", "，"} {
		if idx := strings.Index(content, sep); idx > 0 {
			title = strings.TrimSpace(content[:idx])
			break
		}
	}
	if r := []rune(title); len(r) > 80 {
		title = string(r[:80]) + "…"
	}
	return title
}

// ocrConfigMu 保护 ~/.opencodereview/config.json 的读-改-写。
// 此前并发评审/agent 各自 read-modify-write 同一文件，不同 language 互相覆盖
// 甚至写出损坏的 JSON。加锁后串行化，避免丢失更新与撕裂写。
// 注意：锁只保证不损坏，不同 language 的并发评审仍会「最后写入者胜」——
// 彻底解决需改为按评审传入 language（CLI flag/env），属更大改造。
var ocrConfigMu sync.Mutex

// ensureOCRLanguage 将 language 写入 ~/.opencodereview/config.json。
func ensureOCRLanguage(binary, language string) {
	if language == "" {
		return
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	dir := filepath.Join(home, ".opencodereview")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		logx.Warnf("ensureOCRLanguage: create config dir %q failed: %v", dir, err)
		return
	}
	path := filepath.Join(dir, "config.json")

	ocrConfigMu.Lock()
	defer ocrConfigMu.Unlock()

	var cfg map[string]any
	if data, err := os.ReadFile(path); err == nil {
		if err := json.Unmarshal(data, &cfg); err != nil {
			logx.Warnf("ensureOCRLanguage: parse existing config %q failed, will overwrite: %v", path, err)
		}
	}
	if cfg == nil {
		cfg = make(map[string]any)
	}
	if cfg["language"] == language {
		return
	}
	cfg["language"] = language
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		logx.Warnf("ensureOCRLanguage: write config %q failed: %v", path, err)
	}
}
