package runner

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/yi-nology/code-review-service/internal/service/progress"
	"github.com/yi-nology/code-review-service/internal/service/review"
	"github.com/oekit/concurrency/async"
	"github.com/oekit/pkg/convert"
	"github.com/cloudwego/hertz/pkg/common/hlog"
)

type OpenCodeReviewRunner struct {
	config OpenCodeReviewConfig
}

func (r *OpenCodeReviewRunner) Config() OpenCodeReviewConfig {
	return r.config
}

const defaultOCRMaxRetries = 1

func (r *OpenCodeReviewRunner) Run(ctx context.Context, workspace review.Workspace) (review.Result, error) {
	if strings.TrimSpace(workspace.Dir) == "" {
		return review.Result{}, fmt.Errorf("open-code-review requires a checked out workspace")
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
			return review.Result{}, err
		}

		attemptCtx, cancel := context.WithTimeout(ctx, timeout+30*time.Second)
		if reporter != nil {
			attemptCtx = progress.WithProgressReporter(attemptCtx, reporter)
		}

		if attempt > 1 {
			hlog.CtxWarnf(ctx, "ocr run failed, retrying attempt %d/%d: %v", attempt-1, maxRetries, lastErr)
			if reporter != nil {
				reporter.ReportAgentStatus("ocr", "retrying", fmt.Sprintf("重试第 %d 次", attempt-1))
			}
		}

		result, err := r.runOnce(attemptCtx, workspace)
		cancel()
		if err == nil {
			return result, nil
		}
		lastErr = err

		if ctx.Err() != nil {
			break
		}
	}
	return review.Result{}, lastErr
}

func (r *OpenCodeReviewRunner) runOnce(ctx context.Context, workspace review.Workspace) (review.Result, error) {
	fromRef := workspace.BaseSHA
	toRef := workspace.HeadSHA
	if fromRef == "" {
		fromRef = "HEAD"
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
	if strings.TrimSpace(r.config.ToolsPath) != "" {
		args = append(args, "--tools", r.config.ToolsPath)
	}
	if strings.TrimSpace(r.config.Background) != "" {
		args = append(args, "--background", r.config.Background)
	}

	cmd := exec.CommandContext(ctx, r.config.Binary, args...)
	cmd.Dir = workspace.Dir
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	env := os.Environ()
	if r.config.URL != "" {
		env = append(env, "OCR_LLM_URL="+r.config.URL)
	}
	if r.config.Model != "" {
		env = append(env, "OCR_LLM_MODEL="+r.config.Model)
	}
	if r.config.AuthToken != "" {
		env = append(env, "OCR_LLM_TOKEN="+r.config.AuthToken)
	}
	if r.config.AuthHeader != "" {
		env = append(env, "OCR_LLM_AUTH_HEADER="+r.config.AuthHeader)
	}
	if r.config.UseAnthropic {
		env = append(env, "OCR_USE_ANTHROPIC=true")
	}
	if r.config.Language != "" {
		env = append(env, "OCR_LANGUAGE="+r.config.Language)
	}
	if r.config.ExtraBody != "" {
		env = append(env, "OCR_LLM_EXTRA_BODY="+r.config.ExtraBody)
	}
	cmd.Env = env

	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return review.Result{}, fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	progressReporter := progress.ProgressReporterFromContext(ctx)

	if err := cmd.Start(); err != nil {
		return review.Result{}, fmt.Errorf("open-code-review start failed: %s", err.Error())
	}

	var stderrLines []string
	var stderrMu sync.Mutex
	async.GoSafe(func() {
		scanner := bufio.NewScanner(stderrPipe)
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
			_ = err
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
	if err != nil {
		if cmd.Process != nil {
			_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		}
		stderrMu.Lock()
		message := strings.Join(stderrLines, "\n")
		stderrMu.Unlock()
		message = strings.TrimSpace(message)
		if message == "" {
			message = err.Error()
		}
		return review.Result{}, fmt.Errorf("open-code-review failed: %s", message)
	}

	return ParseOpenCodeReviewOutput(stdout.Bytes(), r.config.DefaultSeverity)
}

type DeterministicRunner struct{}

func NewDeterministicRunner() *DeterministicRunner {
	return &DeterministicRunner{}
}

func (r *DeterministicRunner) Run(_ context.Context, workspace review.Workspace) (review.Result, error) {
	summary := fmt.Sprintf("review completed for %s at %s", workspace.Repo, shortSHA(workspace.HeadSHA))
	if workspace.Diff != "" {
		summary = fmt.Sprintf("%s (%d diff bytes)", summary, len(workspace.Diff))
	}
	return review.Result{Summary: summary, Findings: []review.Finding{}}, nil
}

type CommandRunner struct {
	command  string
	fallback review.Runner
}

func NewCommandRunner(command string, fallback review.Runner) *CommandRunner {
	return &CommandRunner{command: command, fallback: fallback}
}

func (r *CommandRunner) Run(ctx context.Context, workspace review.Workspace) (review.Result, error) {
	parts := strings.Fields(r.command)
	if len(parts) == 0 {
		return r.fallback.Run(ctx, workspace)
	}

	cmd := exec.CommandContext(ctx, parts[0], parts[1:]...)
	cmd.Env = append(os.Environ(),
		"REVIEW_PROVIDER="+workspace.Provider,
		"REVIEW_REPO="+workspace.Repo,
		"REVIEW_BASE_SHA="+workspace.BaseSHA,
		"REVIEW_HEAD_SHA="+workspace.HeadSHA,
		"REVIEW_WORKSPACE_DIR="+workspace.Dir,
	)
	cmd.Stdin = strings.NewReader(workspace.Diff)

	output, err := cmd.Output()
	if err != nil {
		return review.Result{}, err
	}
	if len(strings.TrimSpace(string(output))) == 0 {
		return r.fallback.Run(ctx, workspace)
	}

	var result review.Result
	if err := json.Unmarshal(output, &result); err != nil {
		return review.Result{}, fmt.Errorf("parse OCR output: %w", err)
	}
	if result.Summary == "" {
		result.Summary = "review completed"
	}
	return result, nil
}

func shortSHA(sha string) string {
	if len(sha) <= 8 {
		return sha
	}
	return sha[:8]
}
