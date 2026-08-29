package ocr

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"syscall"

	"github.com/yi-nology/code-review-service/internal/service/review/domain"

	"github.com/oekit/concurrency/async"
	"github.com/oekit/logx"
)

// Preview 实现 domain.PreviewRunner（吸收 runner/preview.go）。
//
// 调 "ocr scan --preview" 子进程，返回将参与评审的文件列表。
// 注意：ocr --preview 不支持 --format json，输出为带 ANSI 颜色的纯文本，
// 需要从文本中提取 "Will review" 区块的文件路径。
func (r *Runner) Preview(ctx context.Context, workspace domain.Workspace) (*domain.PreviewResult, error) {
	if strings.TrimSpace(workspace.Dir) == "" {
		return nil, fmt.Errorf("需要已检出的工作区")
	}

	cfg := r.effectiveConfig(workspace.OCR)

	args := []string{
		"scan", "--preview",
		"--repo", workspace.Dir,
	}
	if len(workspace.ScanPaths) > 0 {
		args = append(args, "--path", strings.Join(workspace.ScanPaths, ","))
	}
	if len(workspace.ScanExclude) > 0 {
		args = append(args, "--exclude", strings.Join(workspace.ScanExclude, ","))
	}
	toolsArgs, cleanupTools, err := appendToolsArgs(args, cfg)
	if err != nil {
		return nil, err
	}
	args = toolsArgs
	if cleanupTools != nil {
		defer cleanupTools()
	}

	cmd := exec.CommandContext(ctx, cfg.Binary, args...)
	cmd.Dir = workspace.Dir
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Env = cfg.AppendEnv(os.Environ())

	stdout := newCappedBuffer(maxSubprocessOutput)
	stderr := newCappedBuffer(maxSubprocessOutput)
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	if err := cmd.Start(); err != nil {
		return nil, &BinaryMissingError{Binary: cfg.Binary, Err: err}
	}

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
		return nil, fmt.Errorf("ocr preview 失败: %s", strings.TrimSpace(stderr.String()))
	}

	// 返回去 ANSI 后的原始文本行，由前端解析 "Will review" / "Excluded" 分段。
	// 前端 RepositoryScan.vue 和 ScanPreviewPanel.vue 各有解析逻辑。
	rawText := ansiStrip(stdout.String())
	var files []string
	for _, line := range strings.Split(rawText, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			files = append(files, line)
		}
	}
	return &domain.PreviewResult{Files: files}, nil
}

// ansiStrip 去除 ANSI 转义序列（颜色码等）。
var ansiRe = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func ansiStrip(s string) string {
	return ansiRe.ReplaceAllString(s, "")
}

// Scan 实现 domain.ScanRunner（吸收 runner/scan.go）。
//
// 调 "ocr scan" 子进程（不带 --preview），返回全量扫描结果。
// 支持 batch / paths / exclude / background / tools 等参数。
func (r *Runner) Scan(ctx context.Context, workspace domain.Workspace, req domain.ScanRequest) (*domain.ScanResult, error) {
	// 从 DB 动态解析最新配置（含 binary、language 等），合并 workspace 的项目级 overrides。
	cfg := r.effectiveConfig(workspace.OCR)

	args := []string{
		"scan",
		"--repo", workspace.Dir,
		"--format", "json",
	}
	if req.Batch != "" {
		args = append(args, "--batch", req.Batch)
	}
	if len(req.Paths) > 0 {
		args = append(args, "--path", strings.Join(req.Paths, ","))
	}
	if len(req.Exclude) > 0 {
		args = append(args, "--exclude", strings.Join(req.Exclude, ","))
	}
	bg := req.Background
	if strings.TrimSpace(bg) == "" {
		bg = cfg.Background
	}
	if strings.TrimSpace(bg) != "" {
		args = append(args, "--background", bg)
	}
	// 将语言要求注入 background，确保 LLM 用指定语言输出。
	lang := cfg.Language
	if lang == "" {
		lang = "中文"
	}
	langPrompt := languagePrompt(lang)
	background := strings.TrimSpace(bg)
	if langPrompt != "" {
		if background != "" {
			background = background + "\n\n" + langPrompt
		} else {
			background = langPrompt
		}
	}
	if background != "" {
		args = append(args, "--background", background)
	}
	// OCR v1.7.6+ --background-file：同 runner.go 的处理，路径必须落在 workspace.Dir 内。
	if bgFile, ok := safeBackgroundFile(workspace.Dir, cfg.BackgroundFile); ok {
		args = append(args, "--background-file", bgFile)
	} else if strings.TrimSpace(cfg.BackgroundFile) != "" {
		logx.Warnf("[ocr] background_file %q 不在工作区 %q 内或不存在，已忽略", cfg.BackgroundFile, workspace.Dir)
	}
	toolsArgs, cleanupTools, err := appendToolsArgs(args, cfg)
	if err != nil {
		return nil, err
	}
	args = toolsArgs
	if cleanupTools != nil {
		defer cleanupTools()
	}

	cmd := exec.CommandContext(ctx, cfg.Binary, args...)
	cmd.Dir = workspace.Dir
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Env = cfg.AppendEnv(os.Environ())

	stdout := newCappedBuffer(maxSubprocessOutput)
	stderr := newCappedBuffer(maxSubprocessOutput)
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	if err := cmd.Start(); err != nil {
		return nil, &BinaryMissingError{Binary: cfg.Binary, Err: err}
	}

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
		return nil, fmt.Errorf("ocr scan 失败: %s", strings.TrimSpace(stderr.String()))
	}

	// OCR scan 输出格式与 review 相同（{status, summary, comments}），
	// 用 parseOpenCodeReviewOutput 转换为 domain.Result 再包装成 ScanResult。
	// 不能直接 json.Unmarshal 到 ScanResult——那会因字段不匹配导致 Result 为 nil。
	severity := cfg.DefaultSeverity
	if strings.TrimSpace(severity) == "" {
		severity = "major"
	}
	result := parseOpenCodeReviewOutput(stdout.Bytes(), severity)
	return &domain.ScanResult{Result: &result}, nil
}
