package runner

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"github.com/yi-nology/code-review-service/internal/service/review"
	"github.com/oekit/concurrency/async"
	"github.com/oekit/pkg/convert"
)

func (r *DynamicOpenCodeReviewRunner) Scan(ctx context.Context, workspace review.Workspace, req review.ScanRequest) (*review.ScanResult, error) {
	s, err := r.settings.Get(ctx)
	if err != nil {
		return nil, fmt.Errorf("读取 OCR 配置失败: %w", err)
	}
	enabled := s.Enabled
	if workspace.OCR.Enabled != nil {
		enabled = *workspace.OCR.Enabled
	}
	if !enabled {
		return nil, fmt.Errorf("代码评审未启用，请先在 OCR 配置页面启用并测试模型连接")
	}

	cfg := r.buildConfig(s, workspace.OCR)
	ocr := &OpenCodeReviewRunner{config: cfg}
	return ocr.Scan(ctx, workspace, req)
}

func (r *OpenCodeReviewRunner) Scan(ctx context.Context, workspace review.Workspace, req review.ScanRequest) (*review.ScanResult, error) {
	if strings.TrimSpace(workspace.Dir) == "" {
		return nil, fmt.Errorf("open-code-review scan requires a checked out workspace")
	}

	timeout := time.Duration(r.config.TimeoutMinutes) * time.Minute
	if timeout <= 0 {
		timeout = 10 * time.Minute
	}
	ctx, cancel := context.WithTimeout(ctx, timeout+30*time.Second)
	defer cancel()

	args := []string{
		"scan",
		"--repo", workspace.Dir,
		"--format", "json",
		"--concurrency", convert.IntToStr(r.config.Concurrency),
		"--max-git-procs", convert.IntToStr(r.config.MaxGitProcs),
		"--timeout", convert.IntToStr(r.config.TimeoutMinutes),
		"--audience", r.config.Audience,
	}
	if req.Preview {
		args = append(args, "--preview")
	}
	if len(req.Paths) > 0 {
		args = append(args, "--path", strings.Join(cleanStrings(req.Paths), ","))
	}
	if len(req.Exclude) > 0 {
		args = append(args, "--exclude", strings.Join(cleanStrings(req.Exclude), ","))
	}
	if req.MaxTokensBudget > 0 {
		args = append(args, "--max-tokens-budget", convert.IntToStr(req.MaxTokensBudget))
	}
	if strings.TrimSpace(req.Batch) != "" {
		args = append(args, "--batch", strings.TrimSpace(req.Batch))
	}
	if req.NoPlan {
		args = append(args, "--no-plan")
	}
	if req.NoDedup {
		args = append(args, "--no-dedup")
	}
	if req.NoSummary {
		args = append(args, "--no-summary")
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
		return nil, err
	}
	args = toolsArgs
	if cleanupTools != nil {
		defer cleanupTools()
	}
	background := firstNonBlank(req.Background, r.config.Background)
	if strings.TrimSpace(background) != "" {
		args = append(args, "--background", background)
	}

	cmd := exec.CommandContext(ctx, r.config.Binary, args...)
	cmd.Dir = workspace.Dir
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	env := os.Environ()
	env = AppendLLMEnv(env, r.config)
	cmd.Env = env

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

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

	err = cmd.Run()
	close(done)
	if err != nil {
		if cmd.Process != nil {
			_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		}
		message := strings.TrimSpace(stderr.String())
		return nil, classifyExitError(ctx, err, timeout, message)
	}

	output := bytes.TrimSpace(stdout.Bytes())
	if req.Preview {
		return &review.ScanResult{PreviewFiles: parsePreviewFiles(output)}, nil
	}

	result, err := ParseOpenCodeReviewOutput(output, r.config.DefaultSeverity)
	if err != nil {
		return nil, err
	}
	if result.Scope.ReviewedFiles == 0 && result.OCR.FilesReviewed > 0 {
		result.Scope.ReviewedFiles = int(result.OCR.FilesReviewed)
		result.Scope.ChangedFiles = int(result.OCR.FilesReviewed)
	}
	report := review.BuildReport(&result)
	analysis := review.BuildAnalysis(result)
	return &review.ScanResult{
		Result:   &result,
		Report:   &report,
		Analysis: &analysis,
		Warnings: result.Warnings,
	}, nil
}

func cleanStrings(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			out = append(out, value)
		}
	}
	return out
}

func parsePreviewFiles(output []byte) []string {
	if len(output) == 0 {
		return []string{}
	}
	var files []string
	if err := json.Unmarshal(output, &files); err == nil {
		return files
	}
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	files = make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			files = append(files, line)
		}
	}
	return files
}

var _ review.ScanRunner = (*OpenCodeReviewRunner)(nil)
var _ review.ScanRunner = (*DynamicOpenCodeReviewRunner)(nil)
