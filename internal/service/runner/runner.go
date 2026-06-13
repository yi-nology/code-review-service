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

	"github.com/oekit/concurrency/async"
	"review-service/internal/repository/postgres/models"
	"review-service/internal/service/review"
)

type OpenCodeReviewConfig struct {
	Binary          string
	RulePath        string
	DefaultSeverity string
	Concurrency     int
	MaxGitProcs     int
	TimeoutMinutes  int
	MaxTools        int
	URL             string
	Model           string
	AuthToken       string
	AuthHeader      string
	UseAnthropic    bool
	ExtraBody       string
	Language        string
}

type OpenCodeReviewRunner struct {
	config OpenCodeReviewConfig
}

func NewOpenCodeReviewRunner(config OpenCodeReviewConfig) *OpenCodeReviewRunner {
	if strings.TrimSpace(config.Binary) == "" {
		config.Binary = "ocr"
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

func (r *OpenCodeReviewRunner) Run(ctx context.Context, workspace review.Workspace) (review.Result, error) {
	if strings.TrimSpace(workspace.Dir) == "" {
		return review.Result{}, fmt.Errorf("open-code-review requires a checked out workspace")
	}

	timeout := time.Duration(r.config.TimeoutMinutes) * time.Minute
	if timeout <= 0 {
		timeout = 10 * time.Minute
	}
	ctx, cancel := context.WithTimeout(ctx, timeout+30*time.Second)
	defer cancel()

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
		"--concurrency", fmt.Sprintf("%d", r.config.Concurrency),
		"--max-git-procs", fmt.Sprintf("%d", r.config.MaxGitProcs),
		"--timeout", fmt.Sprintf("%d", r.config.TimeoutMinutes),
	}
	rulePath := r.config.RulePath
	if strings.TrimSpace(workspace.RulePath) != "" {
		rulePath = workspace.RulePath
	}
	if strings.TrimSpace(rulePath) != "" {
		args = append(args, "--rule", rulePath)
	}
	if r.config.MaxTools > 0 {
		args = append(args, "--max-tools", fmt.Sprintf("%d", r.config.MaxTools))
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
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Start the process (don't block yet)
	if err := cmd.Start(); err != nil {
		return review.Result{}, fmt.Errorf("open-code-review start failed: %s", err.Error())
	}

	// Goroutine to kill process group when context times out.
	// The done channel prevents killing the PID after normal exit
	// (deferred cancel fires even on success, risking PID reuse).
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

	err := cmd.Wait()
	close(done)
	if err != nil {
		// Kill the entire process group on error (covers child processes)
		if cmd.Process != nil {
			_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		}
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = err.Error()
		}
		return review.Result{}, fmt.Errorf("open-code-review failed: %s", message)
	}

	return ParseOpenCodeReviewOutput(stdout.Bytes(), r.config.DefaultSeverity)
}

type OCRSettingsSource interface {
	Get(ctx context.Context) (*models.OCRSettings, error)
}

type DynamicOpenCodeReviewRunner struct {
	fallback *OpenCodeReviewRunner
	settings OCRSettingsSource
}

func NewDynamicOpenCodeReviewRunner(fallback *OpenCodeReviewRunner, settings OCRSettingsSource) *DynamicOpenCodeReviewRunner {
	return &DynamicOpenCodeReviewRunner{fallback: fallback, settings: settings}
}

func (r *DynamicOpenCodeReviewRunner) Run(ctx context.Context, workspace review.Workspace) (review.Result, error) {
	s, err := r.settings.Get(ctx)
	if err != nil {
		return review.Result{}, fmt.Errorf("读取 OCR 配置失败: %w", err)
	}
	if !s.Enabled {
		return review.Result{}, fmt.Errorf("代码评审未启用，请先在 OCR 配置页面启用并测试模型连接")
	}

	cfg := r.buildConfig(s)
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

	cfg := r.buildConfig(s)

	args := []string{"llm", "test"}
	cmd := exec.CommandContext(ctx, cfg.Binary, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	env := os.Environ()
	env = r.appendLLMEnv(env, cfg)
	cmd.Env = env

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return &review.LLMTestResult{Success: false, Message: msg}, nil
	}

	output := strings.TrimSpace(stdout.String())
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

	cfg := r.buildConfig(s)
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

func (r *DynamicOpenCodeReviewRunner) buildConfig(s *models.OCRSettings) OpenCodeReviewConfig {
	cfg := OpenCodeReviewConfig{
		Binary:          r.fallback.config.Binary,
		RulePath:        r.fallback.config.RulePath,
		DefaultSeverity: r.fallback.config.DefaultSeverity,
		URL:             s.URL,
		Model:           s.ModelName,
		AuthToken:       s.AuthToken,
		AuthHeader:      s.AuthHeader,
		UseAnthropic:    s.UseAnthropic,
		ExtraBody:       string(s.ExtraBody),
		Language:        s.Language,
		MaxTools:        s.MaxTools,
	}
	if s.Concurrency > 0 {
		cfg.Concurrency = s.Concurrency
	} else {
		cfg.Concurrency = r.fallback.config.Concurrency
	}
	if s.MaxGitProcs > 0 {
		cfg.MaxGitProcs = s.MaxGitProcs
	} else {
		cfg.MaxGitProcs = r.fallback.config.MaxGitProcs
	}
	if s.TimeoutMinutes > 0 {
		cfg.TimeoutMinutes = s.TimeoutMinutes
	} else {
		cfg.TimeoutMinutes = r.fallback.config.TimeoutMinutes
	}
	return cfg
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
	return env
}
