package runner

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"regexp"
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

func (r *DynamicOpenCodeReviewRunner) Scan(ctx context.Context, workspace domain.Workspace, req domain.ScanRequest) (*domain.ScanResult, error) {
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
	if strings.TrimSpace(cfg.ToolsPath) == "" && strings.TrimSpace(cfg.ToolsConfig) == "" {
		cfg.ToolsConfig = DefaultToolConfig(r.toolPackages, "scan")
	}
	ocr := &OpenCodeReviewRunner{config: cfg}
	return ocr.Scan(ctx, workspace, req)
}

func (r *OpenCodeReviewRunner) Scan(ctx context.Context, workspace domain.Workspace, req domain.ScanRequest) (*domain.ScanResult, error) {
	if strings.TrimSpace(workspace.Dir) == "" {
		return nil, fmt.Errorf("open-code-review scan requires a checked out workspace")
	}

	timeout := time.Duration(r.config.TimeoutMinutes) * time.Minute
	if timeout <= 0 {
		timeout = 10 * time.Minute
	}
	progressReporter := progress.ProgressReporterFromContext(ctx)

	ctx, cancel := context.WithTimeout(ctx, timeout+30*time.Second)
	defer cancel()
	if progressReporter != nil {
		ctx = progress.WithProgressReporter(ctx, progressReporter)
	}

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
	cmd.Stdout = &stdout

	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, &BinaryMissingError{Binary: r.config.Binary, Err: err}
	}

	if progressReporter != nil {
		progressReporter.UpdatePhase(progress.PhaseAnalysis, "active", 15, "正在扫描代码，请稍候...")
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
		return nil, classifyExitError(ctx, err, timeout, message)
	}

	output := bytes.TrimSpace(stdout.Bytes())
	if req.Preview {
		return &domain.ScanResult{PreviewFiles: parsePreviewFiles(output)}, nil
	}

	result, err := ParseOpenCodeReviewOutput(output, r.config.DefaultSeverity)
	if err != nil {
		return nil, err
	}
	if result.Scope.ReviewedFiles == 0 && result.OCR.FilesReviewed > 0 {
		result.Scope.ReviewedFiles = int(result.OCR.FilesReviewed)
		result.Scope.ChangedFiles = int(result.OCR.FilesReviewed)
	}
	return &domain.ScanResult{
		Result:   &result,
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
		return cleanPreviewFiles(files)
	}
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	files = make([]string, 0, len(lines))
	inReviewSection := false
	for _, raw := range lines {
		line := strings.TrimSpace(stripANSI(raw))
		if line == "" {
			continue
		}
		lower := strings.ToLower(line)
		switch {
		case strings.HasPrefix(lower, "will review"):
			inReviewSection = true
			continue
		case strings.HasPrefix(lower, "excluded from review"):
			inReviewSection = false
			continue
		case strings.HasPrefix(lower, "preview:"):
			continue
		}
		if inReviewSection {
			if file := parsePreviewFileLine(line); file != "" {
				files = append(files, file)
			}
		}
	}
	if len(files) > 0 {
		return files
	}
	return cleanPreviewFiles(lines)
}

var ansiPattern = regexp.MustCompile(`\x1b\[[0-9;?]*[ -/]*[@-~]`)

func stripANSI(value string) string {
	return ansiPattern.ReplaceAllString(value, "")
}

func parsePreviewFileLine(line string) string {
	line = strings.TrimSpace(line)
	if strings.HasPrefix(line, "[") {
		if end := strings.Index(line, "]"); end >= 0 {
			line = strings.TrimSpace(line[end+1:])
		}
	}
	if idx := strings.Index(line, " ("); idx >= 0 {
		line = strings.TrimSpace(line[:idx])
	}
	if idx := strings.Index(line, " +"); idx >= 0 {
		line = strings.TrimSpace(line[:idx])
	}
	if idx := strings.Index(line, " -"); idx >= 0 {
		line = strings.TrimSpace(line[:idx])
	}
	return line
}

func cleanPreviewFiles(values []string) []string {
	files := make([]string, 0, len(values))
	for _, value := range values {
		value = parsePreviewFileLine(stripANSI(value))
		lower := strings.ToLower(value)
		if value == "" || strings.HasPrefix(lower, "preview:") || strings.HasPrefix(lower, "will review") || strings.HasPrefix(lower, "excluded from review") {
			continue
		}
		files = append(files, value)
	}
	return files
}

var _ domain.ScanRunner = (*OpenCodeReviewRunner)(nil)
var _ domain.ScanRunner = (*DynamicOpenCodeReviewRunner)(nil)
