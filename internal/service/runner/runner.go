package runner

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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

type OpenCodeReviewRunner struct {
	config OpenCodeReviewConfig
}

func (r *OpenCodeReviewRunner) Config() OpenCodeReviewConfig {
	return r.config
}

// defaultOCRMaxRetries 默认不重试：LLM endpoint 不可达 / DNS 失败 / 配置错误这类
// 不可重试的错误重试只会成倍放大超时窗口（默认每次 10.5min）。需要重试时显式配置。
const defaultOCRMaxRetries = 0

func (r *OpenCodeReviewRunner) Run(ctx context.Context, workspace domain.Workspace) (domain.Result, error) {
	if strings.TrimSpace(workspace.Dir) == "" {
		return domain.Result{}, fmt.Errorf("open-code-review requires a checked out workspace")
	}

	timeout := time.Duration(r.config.TimeoutMinutes) * time.Minute
	if timeout <= 0 {
		timeout = 10 * time.Minute
	}

	maxRetries := r.config.MaxRetries
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

func (r *OpenCodeReviewRunner) runOnce(ctx context.Context, workspace domain.Workspace, timeout time.Duration) (domain.Result, error) {
	fromRef := workspace.BaseSHA
	toRef := workspace.HeadSHA
	if fromRef == "" {
		fromRef = "HEAD"
	}

	if lang := strings.TrimSpace(r.config.Language); lang != "" {
		ensureOCRLanguage(r.config.Binary, lang)
	}

	args := []string{
		"review",
		"--repo", workspace.Dir,
		"--from", fromRef,
		"--to", toRef,
		"--format", "json",
		"--concurrency", convert.IntToStr(r.config.Concurrency),
		"--max-git-procs", convert.IntToStr(r.config.MaxGitProcs),
		"--timeout", convert.IntToStr(r.config.TimeoutMinutes),
		"--audience", r.config.Audience,
	}
	rulePath := r.config.RulePath
	if strings.TrimSpace(workspace.RulePath) != "" {
		rulePath = workspace.RulePath
	}
	if strings.TrimSpace(rulePath) != "" {
		args = append(args, "--rule", rulePath)
	}
	if r.config.MaxTools > 0 {
		args = append(args, "--max-tools", convert.IntToStr(r.config.MaxTools))
	}
	toolsArgs, cleanupTools, err := appendToolsArgs(args, r.config)
	if err != nil {
		return domain.Result{}, err
	}
	args = toolsArgs
	if cleanupTools != nil {
		defer cleanupTools()
	}
	if strings.TrimSpace(r.config.Background) != "" {
		args = append(args, "--background", r.config.Background)
	}

	cmd := exec.CommandContext(ctx, r.config.Binary, args...)
	cmd.Dir = workspace.Dir
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	env := os.Environ()
	env = AppendLLMEnv(env, r.config)
	cmd.Env = env

	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return domain.Result{}, fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	progressReporter := progress.ProgressReporterFromContext(ctx)

	if err := cmd.Start(); err != nil {
		return domain.Result{}, &BinaryMissingError{Binary: r.config.Binary, Err: err}
	}

	var stderrLines []string
	var stderrMu sync.Mutex
	var stderrWg sync.WaitGroup
	stderrWg.Add(1)
	async.GoSafe(func() {
		defer stderrWg.Done()
		scanner := bufio.NewScanner(stderrPipe)
		// 扩大缓冲到 1MB，避免 OCR 输出的长 JSON/堆栈（>64KB 默认限制）被截断丢弃，
		// 导致失败时拿不到根因（如 LLM 错误堆栈）。
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
	stderrWg.Wait() // 等 stderr goroutine 读完，避免拿到被截断的 stderr（超时/取消场景尤其关键）
	if err != nil {
		if cmd.Process != nil {
			_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		}
		stderrMu.Lock()
		message := strings.Join(stderrLines, "\n")
		stderrMu.Unlock()
		return domain.Result{}, classifyExitError(ctx, err, timeout, message)
	}

	return ParseOpenCodeReviewOutput(stdout.Bytes(), r.config.DefaultSeverity)
}

// ensureOCRLanguage 将 language 写入 ocr 二进制的配置文件（~/.opencodereview/config.json）。
// ocr 二进制不读 OCR_LANGUAGE 环境变量，只读配置文件中的 language 字段，
// 用于在 system prompt 末尾追加 "Always respond in <Language>." 指令。
func ensureOCRLanguage(binary, language string) {
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	dir := filepath.Join(home, ".opencodereview")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return
	}
	path := filepath.Join(dir, "config.json")

	var cfg map[string]any
	if data, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(data, &cfg)
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
	_ = os.WriteFile(path, data, 0o644)
}
