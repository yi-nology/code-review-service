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

	"github.com/yi-nology/code-review-service/internal/repository/postgres/models"
	"github.com/yi-nology/code-review-service/internal/service/progress"
	"github.com/yi-nology/code-review-service/internal/service/review"
	"github.com/oekit/concurrency/async"
	"github.com/oekit/pkg/convert"
	"github.com/cloudwego/hertz/pkg/common/hlog"
)

type OpenCodeReviewConfig struct {
	Binary                  string
	Audience                string
	RulePath                string
	DefaultSeverity         string
	Concurrency             int
	MaxGitProcs             int
	TimeoutMinutes          int
	MaxRetries              int
	MaxTools                int
	URL                     string
	Model                   string
	AuthToken               string
	AuthHeader              string
	UseAnthropic            bool
	ExtraBody               string
	Language                string
	Background              string
	ToolsPath               string
	TelemetryEnabled        bool
	TelemetryExporter       string
	TelemetryOTLPEndpoint   string
	TelemetryContentLogging bool
}

type OpenCodeReviewRunner struct {
	config OpenCodeReviewConfig
}

func NewOpenCodeReviewRunner(config OpenCodeReviewConfig) *OpenCodeReviewRunner {
	if strings.TrimSpace(config.Binary) == "" {
		config.Binary = "ocr"
	}
	if strings.TrimSpace(config.Audience) == "" {
		config.Audience = "human"
	}
	if strings.TrimSpace(config.DefaultSeverity) == "" {
		config.DefaultSeverity = string(review.SeverityMajor)
	}
	if config.Concurrency <= 0 {
		config.Concurrency = 2
	}
	if config.MaxGitProcs <= 0 {
		config.MaxGitProcs = 4
	}
	if config.TimeoutMinutes <= 0 {
		config.TimeoutMinutes = 10
	}
	return &OpenCodeReviewRunner{config: config}
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

	// Create pipe for stderr to capture progress in real-time
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return review.Result{}, fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	// Get progress reporter from context
	progressReporter := progress.ProgressReporterFromContext(ctx)

	// Start the process (don't block yet)
	if err := cmd.Start(); err != nil {
		return review.Result{}, fmt.Errorf("open-code-review start failed: %s", err.Error())
	}

	// Goroutine to read stderr and parse progress
	var stderrLines []string
	var stderrMu sync.Mutex
	async.GoSafe(func() {
		scanner := bufio.NewScanner(stderrPipe)
		for scanner.Scan() {
			line := scanner.Text()
			stderrMu.Lock()
			stderrLines = append(stderrLines, line)
			stderrMu.Unlock()

			// Parse progress from stderr
			if progressReporter != nil {
				parseOCRProgress(line, progressReporter)
			}
		}
		if err := scanner.Err(); err != nil {
			// Log scanner error but don't fail the process
			_ = err
		}
	})

	// Goroutine to kill process group when context times out.
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

// parseOCRProgress parses stderr lines for progress information
func parseOCRProgress(line string, reporter progress.ProgressReporter) {
	line = strings.TrimSpace(line)
	if line == "" {
		return
	}

	// Look for percentage patterns like "50%", "Progress: 50%", "50.5%"
	if idx := strings.Index(line, "%"); idx > 0 {
		// Look backwards for the number
		numStr := ""
		for i := idx - 1; i >= 0; i-- {
			c := line[i]
			if (c >= '0' && c <= '9') || c == '.' {
				numStr = string(c) + numStr
			} else if len(numStr) > 0 {
				break
			}
		}
		if numStr != "" {
			progressVal := convert.Str2Float64(numStr, -1)
			if progressVal >= 0 && progressVal <= 100 {
				reporter.ReportProgress(line, progressVal)
				return
			}
		}
	}

	// Look for file processing patterns like:
	// "Processing file 3/10: path/to/file.go"
	// "[3/10] Analyzing path/to/file.go"
	// "✓ file.go (3/10)"
	if strings.Contains(line, "/") {
		parts := strings.Split(line, "/")
		if len(parts) >= 2 {
			for i, part := range parts {
				part = strings.TrimSpace(part)
				numStr := ""
				for _, c := range part {
					if c >= '0' && c <= '9' {
						numStr += string(c)
					}
				}
				if numStr != "" {
					current := convert.Str2Int(numStr, 0)
					if current > 0 && i < len(parts)-1 {
						totalStr := ""
						for _, c := range parts[i+1] {
							if c >= '0' && c <= '9' {
								totalStr += string(c)
							} else if len(totalStr) > 0 {
								break
							}
						}
						if totalStr != "" {
							total := convert.Str2Int(totalStr, 0)
							if total > 0 {
								reporter.ReportFileProgress(line, current, total)
								return
							}
						}
					}
				}
			}
		}
	}

	// Look for agent status patterns
	if strings.Contains(line, "Agent") || strings.Contains(line, "agent") {
		reporter.ReportAgentStatus("ocr", "active", line)
	}
}

type OCRSettingsSource interface {
	Get(ctx context.Context) (*models.OCRSettings, error)
}

type viewerManager struct {
	process *exec.Cmd
	addr    string
	mu      sync.Mutex
	done    chan struct{}
}

type DynamicOpenCodeReviewRunner struct {
	fallback     *OpenCodeReviewRunner
	settings     OCRSettingsSource
	globalRules  GlobalRulesSource
	viewer       *viewerManager
}

type GlobalRulesSource interface {
	Get(ctx context.Context) (*models.GlobalRules, error)
}

func NewDynamicOpenCodeReviewRunner(fallback *OpenCodeReviewRunner, settings OCRSettingsSource, globalRules GlobalRulesSource) *DynamicOpenCodeReviewRunner {
	return &DynamicOpenCodeReviewRunner{fallback: fallback, settings: settings, globalRules: globalRules, viewer: &viewerManager{}}
}

func (r *DynamicOpenCodeReviewRunner) resolveBinaryFrom(s *models.OCRSettings) string {
	if s != nil && s.Binary != "" {
		return s.Binary
	}
	if r.fallback.config.Binary != "" {
		return r.fallback.config.Binary
	}
	return "ocr"
}

type ocrCommandResult struct {
	stdout string
	stderr string
}

func runOCRCommand(ctx context.Context, binary string, args []string, env []string) (*ocrCommandResult, error) {
	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if len(env) > 0 {
		cmd.Env = env
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	return &ocrCommandResult{
		stdout: stdout.String(),
		stderr: stderr.String(),
	}, err
}

func (r *DynamicOpenCodeReviewRunner) Run(ctx context.Context, workspace review.Workspace) (review.Result, error) {
	s, err := r.settings.Get(ctx)
	if err != nil {
		return review.Result{}, fmt.Errorf("读取 OCR 配置失败: %w", err)
	}
	enabled := s.Enabled
	if workspace.OCR.Enabled != nil {
		enabled = *workspace.OCR.Enabled
	}
	if !enabled {
		return review.Result{}, fmt.Errorf("代码评审未启用，请先在 OCR 配置页面启用并测试模型连接")
	}

	cfg := r.buildConfig(s, workspace.OCR)
	ocr := &OpenCodeReviewRunner{config: cfg}
	return ocr.Run(ctx, workspace)
}

type DeterministicRunner struct{}

func NewDeterministicRunner() *DeterministicRunner {
	return &DeterministicRunner{}
}

func (r *DeterministicRunner) Run(_ context.Context, workspace review.Workspace) (review.Result, error) {
	summary := fmt.Sprintf(
		"review completed for %s at %s",
		workspace.Repo,
		shortSHA(workspace.HeadSHA),
	)
	if workspace.Diff != "" {
		summary = fmt.Sprintf("%s (%d diff bytes)", summary, len(workspace.Diff))
	}

	return review.Result{
		Summary:  summary,
		Findings: []review.Finding{},
	}, nil
}

func shortSHA(sha string) string {
	if len(sha) <= 8 {
		return sha
	}
	return sha[:8]
}

type CommandRunner struct {
	command  string
	fallback review.Runner
}

func NewCommandRunner(command string, fallback review.Runner) *CommandRunner {
	return &CommandRunner{
		command:  command,
		fallback: fallback,
	}
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

func ParseOpenCodeReviewOutput(output []byte, defaultSeverity string) (review.Result, error) {
	output = bytes.TrimSpace(output)
	if len(output) == 0 {
		return review.Result{}, fmt.Errorf("open-code-review produced empty output")
	}
	jsonStart := bytes.IndexByte(output, '{')
	if jsonStart < 0 {
		return review.Result{}, fmt.Errorf("open-code-review output does not contain JSON")
	}
	prefix := strings.TrimSpace(string(output[:jsonStart]))
	payload := bytes.TrimSpace(output[jsonStart:])

	var parsed openCodeReviewOutput
	if err := json.Unmarshal(payload, &parsed); err != nil {
		return review.Result{}, fmt.Errorf("parse open-code-review output: %w", err)
	}

	findings := make([]review.Finding, 0, len(parsed.Comments))
	for _, comment := range parsed.Comments {
		line := comment.StartLine
		if line == 0 {
			line = comment.EndLine
		}
		findings = append(findings, review.Finding{
			Severity:       review.InferSeverity(comment.Content, defaultSeverity),
			Path:           comment.Path,
			Line:           line,
			StartLine:      comment.StartLine,
			EndLine:        comment.EndLine,
			Title:          firstLine(comment.Content),
			Body:           strings.TrimSpace(comment.Content),
			ExistingCode:   strings.TrimSpace(comment.ExistingCode),
			SuggestionCode: strings.TrimSpace(comment.SuggestionCode),
		})
	}

	cleaned := sanitizeRawOCR(payload)
	result := review.Result{
		Summary: summaryText(parsed, len(findings)),
		OCR: review.OCRMetadata{
			Engine: "open-code-review",
			Status: parsed.Status,
		},
		Warnings: parseWarnings(parsed.Warnings),
		Findings: findings,
		RawOCR:   append(json.RawMessage(nil), cleaned...),
	}
	if prefix != "" {
		result.Warnings = append([]review.OCRWarning{{
			Type:    "runner_output",
			Message: prefix,
		}}, result.Warnings...)
	}
	if parsed.Summary != nil {
		result.OCR.FilesReviewed = parsed.Summary.FilesReviewed
		result.OCR.Comments = parsed.Summary.Comments
		result.OCR.TotalTokens = parsed.Summary.TotalTokens
		result.OCR.InputTokens = parsed.Summary.InputTokens
		result.OCR.OutputTokens = parsed.Summary.OutputTokens
		result.OCR.CacheReadTokens = parsed.Summary.CacheReadTokens
		result.OCR.CacheWriteTokens = parsed.Summary.CacheWriteTokens
		result.OCR.Elapsed = parsed.Summary.Elapsed
	}
	return result, nil
}

type openCodeReviewOutput struct {
	Status   string                  `json:"status"`
	Message  string                  `json:"message"`
	Summary  *openCodeReviewSummary  `json:"summary"`
	Comments []openCodeReviewComment `json:"comments"`
	Warnings []json.RawMessage       `json:"warnings"`
}

type openCodeReviewSummary struct {
	FilesReviewed    int64  `json:"files_reviewed"`
	Comments         int64  `json:"comments"`
	TotalTokens      int64  `json:"total_tokens"`
	InputTokens      int64  `json:"input_tokens"`
	OutputTokens     int64  `json:"output_tokens"`
	CacheReadTokens  int64  `json:"cache_read_tokens"`
	CacheWriteTokens int64  `json:"cache_write_tokens"`
	Elapsed          string `json:"elapsed"`
}

type openCodeReviewComment struct {
	Path           string `json:"path"`
	Content        string `json:"content"`
	SuggestionCode string `json:"suggestion_code"`
	ExistingCode   string `json:"existing_code"`
	StartLine      int    `json:"start_line"`
	EndLine        int    `json:"end_line"`
	Thinking       string `json:"thinking"`
}

func firstLine(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "代码评审意见"
	}
	line, _, _ := strings.Cut(value, "\n")
	if len([]rune(line)) <= 120 {
		return line
	}
	return string([]rune(line)[:120])
}

func summaryText(parsed openCodeReviewOutput, findings int) string {
	if parsed.Summary != nil {
		return fmt.Sprintf(
			"代码评审完成：发现 %d 个问题，审查 %d 个文件，使用 %d Tokens",
			findings,
			parsed.Summary.FilesReviewed,
			parsed.Summary.TotalTokens,
		)
	}
	if strings.TrimSpace(parsed.Message) != "" {
		return parsed.Message
	}
	return fmt.Sprintf("代码评审完成：发现 %d 个问题", findings)
}

func parseWarnings(rawWarnings []json.RawMessage) []review.OCRWarning {
	warnings := make([]review.OCRWarning, 0, len(rawWarnings))
	for _, raw := range rawWarnings {
		var fields map[string]any
		if err := json.Unmarshal(raw, &fields); err != nil {
			warnings = append(warnings, review.OCRWarning{Message: string(raw)})
			continue
		}
		warning := review.OCRWarning{
			Type:    firstString(fields, "type", "code", "kind"),
			Message: firstString(fields, "message", "error", "reason"),
			Path:    firstString(fields, "path", "file"),
		}
		if warning.Message == "" {
			compact, _ := json.Marshal(fields)
			warning.Message = string(compact)
		}
		warnings = append(warnings, warning)
	}
	return warnings
}

func firstString(fields map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := fields[key].(string); ok && strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func sanitizeRawOCR(payload json.RawMessage) json.RawMessage {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(payload, &raw); err != nil {
		return payload
	}

	commentsRaw, ok := raw["comments"]
	if !ok {
		return payload
	}

	var comments []map[string]json.RawMessage
	if err := json.Unmarshal(commentsRaw, &comments); err != nil {
		return payload
	}

	changed := false
	for i, c := range comments {
		if _, has := c["thinking"]; has {
			delete(c, "thinking")
			comments[i] = c
			changed = true
		}
	}

	if !changed {
		return payload
	}

	cleanedComments, err := json.Marshal(comments)
	if err != nil {
		return payload
	}
	raw["comments"] = cleanedComments

	cleaned, err := json.Marshal(raw)
	if err != nil {
		return payload
	}
	return cleaned
}

type LLMTestResult = review.LLMTestResult

func (r *DynamicOpenCodeReviewRunner) TestLLM(ctx context.Context) (*review.LLMTestResult, error) {
	s, err := r.settings.Get(ctx)
	if err != nil {
		return nil, fmt.Errorf("读取 OCR 配置失败: %w", err)
	}
	if !s.Enabled {
		return &review.LLMTestResult{Success: false, Message: "OCR 未启用"}, nil
	}

	cfg := r.buildConfig(s, review.OCROverrides{})

	args := []string{"llm", "test"}
	env := os.Environ()
	env = r.appendLLMEnv(env, cfg)

	result, err := runOCRCommand(ctx, cfg.Binary, args, env)
	if err != nil {
		msg := strings.TrimSpace(result.stderr)
		if msg == "" {
			msg = err.Error()
		}
		return &review.LLMTestResult{Success: false, Message: msg}, nil
	}

	output := strings.TrimSpace(result.stdout)
	if output == "" {
		output = "连接测试成功"
	}
	return &review.LLMTestResult{Success: true, Message: output}, nil
}

func (r *DynamicOpenCodeReviewRunner) Preview(ctx context.Context, workspace review.Workspace) (*review.PreviewResult, error) {
	s, err := r.settings.Get(ctx)
	if err != nil {
		return nil, fmt.Errorf("读取 OCR 配置失败: %w", err)
	}
	if !s.Enabled {
		return nil, fmt.Errorf("代码评审未启用，请先在 OCR 配置页面启用并测试模型连接")
	}

	cfg := r.buildConfig(s, workspace.OCR)
	if strings.TrimSpace(workspace.Dir) == "" {
		return nil, fmt.Errorf("需要已检出的工作区")
	}

	timeout := time.Duration(cfg.TimeoutMinutes) * time.Minute
	if timeout <= 0 {
		timeout = 10 * time.Minute
	}
	ctx, cancel := context.WithTimeout(ctx, timeout+30*time.Second)
	defer cancel()

	fromRef := workspace.BaseSHA
	if fromRef == "" {
		fromRef = "HEAD"
	}

	args := []string{
		"review",
		"--preview",
		"--format", "json",
		"--repo", workspace.Dir,
		"--from", fromRef,
		"--to", workspace.HeadSHA,
	}
	rulePath := cfg.RulePath
	if strings.TrimSpace(workspace.RulePath) != "" {
		rulePath = workspace.RulePath
	}
	if strings.TrimSpace(rulePath) != "" {
		args = append(args, "--rule", rulePath)
	}

	cmd := exec.CommandContext(ctx, cfg.Binary, args...)
	cmd.Dir = workspace.Dir
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	env := os.Environ()
	env = r.appendLLMEnv(env, cfg)
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
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return nil, fmt.Errorf("预览失败: %s", msg)
	}

	output := bytes.TrimSpace(stdout.Bytes())
	if len(output) == 0 {
		return &review.PreviewResult{Files: []string{}}, nil
	}

	var files []string
	if err := json.Unmarshal(output, &files); err != nil {
		lines := strings.Split(strings.TrimSpace(string(output)), "\n")
		files = make([]string, 0, len(lines))
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line != "" {
				files = append(files, line)
			}
		}
	}

	return &review.PreviewResult{Files: files}, nil
}

func (r *DynamicOpenCodeReviewRunner) buildConfig(s *models.OCRSettings, overrides review.OCROverrides) OpenCodeReviewConfig {
	cfg := OpenCodeReviewConfig{
		Binary:                  firstNonBlank(s.Binary, r.fallback.config.Binary),
		Audience:                firstNonBlank(overrides.Audience, s.Audience, r.fallback.config.Audience, "human"),
		RulePath:                r.fallback.config.RulePath,
		DefaultSeverity:         firstNonBlank(overrides.DefaultSeverity, s.DefaultSeverity, r.fallback.config.DefaultSeverity, string(review.SeverityMajor)),
		URL:                     firstNonBlank(overrides.URL, s.URL),
		Model:                   firstNonBlank(overrides.Model, s.ModelName),
		AuthToken:               s.AuthToken,
		AuthHeader:              s.AuthHeader,
		UseAnthropic:            overrides.UseAnthropic || s.UseAnthropic,
		ExtraBody:               firstNonBlank(overrides.ExtraBody, string(s.ExtraBody)),
		Language:                firstNonBlank(overrides.Language, s.Language),
		MaxTools:                firstPositive(overrides.MaxTools, s.MaxTools),
		Background:              firstNonBlank(overrides.Background, s.Background),
		ToolsPath:               firstNonBlank(overrides.ToolsPath, s.ToolsPath),
		TelemetryEnabled:        s.TelemetryEnabled,
		TelemetryExporter:       s.TelemetryExporter,
		TelemetryOTLPEndpoint:   s.TelemetryOTLPEndpoint,
		TelemetryContentLogging: s.TelemetryContentLogging,
	}
	if overrides.Concurrency > 0 {
		cfg.Concurrency = overrides.Concurrency
	} else if s.Concurrency > 0 {
		cfg.Concurrency = s.Concurrency
	} else {
		cfg.Concurrency = r.fallback.config.Concurrency
	}
	if overrides.MaxGitProcs > 0 {
		cfg.MaxGitProcs = overrides.MaxGitProcs
	} else if s.MaxGitProcs > 0 {
		cfg.MaxGitProcs = s.MaxGitProcs
	} else {
		cfg.MaxGitProcs = r.fallback.config.MaxGitProcs
	}
	if overrides.TimeoutMinutes > 0 {
		cfg.TimeoutMinutes = overrides.TimeoutMinutes
	} else if s.TimeoutMinutes > 0 {
		cfg.TimeoutMinutes = s.TimeoutMinutes
	} else {
		cfg.TimeoutMinutes = r.fallback.config.TimeoutMinutes
	}
	return cfg
}

func firstPositive(values ...int) int {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}

func firstNonBlank(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func (r *DynamicOpenCodeReviewRunner) appendLLMEnv(env []string, cfg OpenCodeReviewConfig) []string {
	if cfg.URL != "" {
		env = append(env, "OCR_LLM_URL="+cfg.URL)
	}
	if cfg.Model != "" {
		env = append(env, "OCR_LLM_MODEL="+cfg.Model)
	}
	if cfg.AuthToken != "" {
		env = append(env, "OCR_LLM_TOKEN="+cfg.AuthToken)
	}
	if cfg.AuthHeader != "" {
		env = append(env, "OCR_LLM_AUTH_HEADER="+cfg.AuthHeader)
	}
	if cfg.UseAnthropic {
		env = append(env, "OCR_USE_ANTHROPIC=true")
	} else {
		env = append(env, "OCR_USE_ANTHROPIC=false")
	}
	if cfg.Language != "" {
		env = append(env, "OCR_LANGUAGE="+cfg.Language)
	}
	if cfg.ExtraBody != "" {
		env = append(env, "OCR_LLM_EXTRA_BODY="+cfg.ExtraBody)
	}
	if cfg.TelemetryEnabled {
		env = append(env, "OCR_ENABLE_TELEMETRY=true")
	}
	if cfg.TelemetryContentLogging {
		env = append(env, "OCR_CONTENT_LOGGING=true")
	}
	return env
}

// ==================== LLMProviderLister ====================

func (r *DynamicOpenCodeReviewRunner) ListLLMProviders(ctx context.Context) ([]review.LLMProvider, error) {
	s, err := r.settings.Get(ctx)
	if err != nil {
		return nil, fmt.Errorf("读取 OCR 配置失败: %w", err)
	}

	binary := r.resolveBinaryFrom(s)

	result, err := runOCRCommand(ctx, binary, []string{"llm", "providers"}, nil)
	if err != nil {
		msg := strings.TrimSpace(result.stderr)
		if msg == "" {
			msg = err.Error()
		}
		return nil, fmt.Errorf("获取 Provider 列表失败: %s", msg)
	}

	output := result.stdout
	var providers []review.LLMProvider
	lines := strings.Split(output, "\n")
	inTable := false
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "NAME") && strings.Contains(line, "PROTOCOL") {
			inTable = true
			continue
		}
		if inTable && line == "" {
			break
		}
		if inTable && line != "" && !strings.HasPrefix(line, "----") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				p := review.LLMProvider{
					Name:     fields[0],
					Protocol: fields[1],
				}
				if len(fields) >= 3 {
					p.BaseURL = fields[2]
				}
				providers = append(providers, p)
			}
		}
	}

	return providers, nil
}

// ==================== RulesChecker ====================

func (r *DynamicOpenCodeReviewRunner) RulesCheck(ctx context.Context, path string) (*review.RulesCheckResult, error) {
	s, err := r.settings.Get(ctx)
	if err != nil {
		return nil, fmt.Errorf("读取 OCR 配置失败: %w", err)
	}

	binary := r.resolveBinaryFrom(s)

	args := []string{"rules", "check"}

	if r.globalRules != nil {
		rulePath, cleanup, ruleErr := r.resolveGlobalRuleFile(ctx)
		if ruleErr == nil && rulePath != "" {
			args = append(args, "--rule", rulePath)
			if cleanup != nil {
				defer cleanup()
			}
		}
	}

	args = append(args, path)

	result, err := runOCRCommand(ctx, binary, args, nil)
	if err != nil {
		msg := strings.TrimSpace(result.stderr)
		if msg == "" {
			msg = err.Error()
		}
		return &review.RulesCheckResult{Applicable: false, Message: msg}, nil
	}

	output := strings.TrimSpace(result.stdout)
	return &review.RulesCheckResult{
		Applicable: output != "",
		Rule:       output,
		Message:    "规则匹配成功",
	}, nil
}

func (r *DynamicOpenCodeReviewRunner) resolveGlobalRuleFile(ctx context.Context) (string, func(), error) {
	if r.globalRules == nil {
		return "", nil, nil
	}

	rules, err := r.globalRules.Get(ctx)
	if err != nil || rules == nil || len(rules.Rules) == 0 {
		return "", nil, nil
	}

	f, err := os.CreateTemp("", "ocr-rules-*.json")
	if err != nil {
		return "", nil, err
	}

	ruleJSON := fmt.Sprintf(`{"rules":%s}`, string(rules.Rules))
	if _, err := f.WriteString(ruleJSON); err != nil {
		_ = f.Close()
		_ = os.Remove(f.Name())
		return "", nil, err
	}
	_ = f.Close()

	cleanup := func() { _ = os.Remove(f.Name()) }
	return f.Name(), cleanup, nil
}

// ==================== OCRVersioner ====================

func (r *DynamicOpenCodeReviewRunner) OCRVersion(ctx context.Context) (*review.OCRVersion, error) {
	s, err := r.settings.Get(ctx)
	if err != nil {
		return nil, fmt.Errorf("读取 OCR 配置失败: %w", err)
	}

	binary := r.resolveBinaryFrom(s)

	cmdResult, err := runOCRCommand(ctx, binary, []string{"version"}, nil)
	if err != nil {
		msg := strings.TrimSpace(cmdResult.stderr)
		if msg == "" {
			msg = err.Error()
		}
		return nil, fmt.Errorf("获取版本失败: %s", msg)
	}

	output := strings.TrimSpace(cmdResult.stdout)
	result := &review.OCRVersion{}

	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "open-code-review ") {
			versionStr := strings.TrimPrefix(line, "open-code-review ")
			parts := strings.SplitN(versionStr, " ", 2)
			if len(parts) >= 1 {
				result.Version = parts[0]
			}
			if len(parts) >= 2 {
				rest := parts[1]
				if strings.HasPrefix(rest, "(") {
					end := strings.Index(rest, ")")
					if end > 0 {
						result.Commit = rest[1:end]
					}
				}
			}
		} else if strings.Contains(line, "darwin") || strings.Contains(line, "linux") || strings.Contains(line, "windows") {
			result.Platform = line
		} else if strings.Contains(line, "built at:") {
			result.BuildTime = strings.TrimSpace(strings.TrimPrefix(line, "built at:"))
		}
	}

	return result, nil
}

// ==================== ViewerManager ====================

func (vm *viewerManager) Status() *review.ViewerStatus {
	vm.mu.Lock()
	defer vm.mu.Unlock()

	if vm.process == nil {
		return &review.ViewerStatus{Running: false, Message: "Viewer 未运行"}
	}

	select {
	case <-vm.done:
		vm.process = nil
		return &review.ViewerStatus{Running: false, Message: "Viewer 已停止"}
	default:
	}

	return &review.ViewerStatus{
		Running: true,
		Addr:    vm.addr,
		Message: fmt.Sprintf("Viewer 运行中 (%s)", vm.addr),
	}
}

func (vm *viewerManager) Start(binary string, addr string) (*review.ViewerStatus, error) {
	vm.mu.Lock()
	defer vm.mu.Unlock()

	if vm.process != nil {
		select {
		case <-vm.done:
			vm.process = nil
		default:
			return &review.ViewerStatus{
				Running: true,
				Addr:    vm.addr,
				Message: "Viewer 已在运行",
			}, nil
		}
	}

	args := []string{"viewer"}
	if addr != "" {
		args = append(args, "--addr", addr)
	} else {
		addr = "localhost:5483"
	}

	cmd := exec.Command(binary, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("启动 Viewer 失败: %w", err)
	}

	vm.process = cmd
	vm.addr = addr
	vm.done = make(chan struct{})

	async.GoSafe(func() {
		_ = cmd.Wait()
		close(vm.done)
	})

	return &review.ViewerStatus{
		Running: true,
		Addr:    addr,
		Message: fmt.Sprintf("Viewer 已启动 (%s)", addr),
	}, nil
}

func (vm *viewerManager) Stop() (*review.ViewerStatus, error) {
	vm.mu.Lock()
	defer vm.mu.Unlock()

	if vm.process == nil {
		return &review.ViewerStatus{Running: false, Message: "Viewer 未运行"}, nil
	}

	select {
	case <-vm.done:
		vm.process = nil
		return &review.ViewerStatus{Running: false, Message: "Viewer 已停止"}, nil
	default:
	}

	if err := syscall.Kill(-vm.process.Process.Pid, syscall.SIGTERM); err != nil {
		_ = syscall.Kill(-vm.process.Process.Pid, syscall.SIGKILL)
	}

	select {
	case <-vm.done:
	case <-time.After(3 * time.Second):
		_ = syscall.Kill(-vm.process.Process.Pid, syscall.SIGKILL)
	}

	addr := vm.addr
	vm.process = nil
	vm.addr = ""

	return &review.ViewerStatus{
		Running: false,
		Addr:    addr,
		Message: "Viewer 已停止",
	}, nil
}

func (r *DynamicOpenCodeReviewRunner) ViewerStatus(ctx context.Context) (*review.ViewerStatus, error) {
	return r.viewer.Status(), nil
}

func (r *DynamicOpenCodeReviewRunner) StartViewer(ctx context.Context, addr string) (*review.ViewerStatus, error) {
	s, err := r.settings.Get(ctx)
	if err != nil {
		return nil, fmt.Errorf("读取 OCR 配置失败: %w", err)
	}

	binary := r.resolveBinaryFrom(s)
	return r.viewer.Start(binary, addr)
}

func (r *DynamicOpenCodeReviewRunner) StopViewer(ctx context.Context) (*review.ViewerStatus, error) {
	return r.viewer.Stop()
}
