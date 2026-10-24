package ocr

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"syscall"

	"github.com/yi-nology/code-review-service/internal/domain/review"

	"github.com/oekit/concurrency/async"
	"github.com/oekit/logx"
	"github.com/oekit/pkg/pathx"
)

// DelegatePreview 实现 domain.Delegator（OCR v1.7.11+）。
//
// 调 `ocr delegate preview` 子进程，返回将参与评审的文件列表 + mode/from/to 元数据。
// 与 PreviewRunner 不同：delegate 输出 host-agent 友好的 markdown（含 mode/ref 信息），
// 且不依赖 --preview（后者输出是 ANSI 文本，给前端人看）。
//
// 失败时返回原始 stderr 文本以便排障。
func (r *Runner) DelegatePreview(ctx context.Context, workspace domain.Workspace) (*domain.DelegatePreviewResult, error) {
	if strings.TrimSpace(workspace.Dir) == "" {
		return nil, fmt.Errorf("需要已检出的工作区")
	}

	cfg := r.effectiveConfig(workspace.OCR)

	args := []string{
		"delegate", "preview",
		"--repo", workspace.Dir,
	}
	if from := strings.TrimSpace(workspace.BaseSHA); from != "" {
		args = append(args, "--from", from)
	}
	if to := strings.TrimSpace(workspace.HeadSHA); to != "" {
		args = append(args, "--to", to)
	}
	if len(workspace.ScanExclude) > 0 {
		args = append(args, "--exclude", strings.Join(workspace.ScanExclude, ","))
	}

	stdout, stderr, err := r.runDelegateCmd(ctx, cfg.Binary, args, workspace.Dir, cfg)
	if err != nil {
		return nil, fmt.Errorf("ocr delegate preview 失败: %s", strings.TrimSpace(stderr))
	}
	return parseDelegatePreview(stdout, workspace.BaseSHA, workspace.HeadSHA), nil
}

// DelegateRule 实现 domain.Delegator（OCR v1.7.11+）。
//
// 调 `ocr delegate rule <paths...>`，返回按内容分组的解析规则 markdown。
// paths 为空时返回全量规则。
func (r *Runner) DelegateRule(ctx context.Context, workspace domain.Workspace, paths []string) (*domain.DelegateRuleResult, error) {
	if strings.TrimSpace(workspace.Dir) == "" {
		return nil, fmt.Errorf("需要已检出的工作区")
	}

	safePaths, err := sanitizeDelegatePaths(paths)
	if err != nil {
		return nil, err
	}

	cfg := r.effectiveConfig(workspace.OCR)

	args := []string{
		"delegate", "rule",
		"--repo", workspace.Dir,
	}
	if rulePath := strings.TrimSpace(workspace.RulePath); rulePath != "" {
		args = append(args, "--rule", rulePath)
	}
	// paths 作为位置参数（ocr delegate rule <path...>），已过 sanitizeDelegatePaths 校验。
	args = append(args, safePaths...)

	stdout, stderr, err := r.runDelegateCmd(ctx, cfg.Binary, args, workspace.Dir, cfg)
	if err != nil {
		return nil, fmt.Errorf("ocr delegate rule 失败: %s", strings.TrimSpace(stderr))
	}
	return &domain.DelegateRuleResult{RawOutput: stdout}, nil
}

// sanitizeDelegatePaths 校验 delegate rule 的位置参数 paths。
//
// paths 直接拼入 OCR CLI argv，以 '-' 开头的路径会被解析成命令行选项（argument
// injection），绝对路径或 '..' 逃逸则可读到工作区之外的文件。空条目跳过。
// 返回原顺序的干净路径列表（不做 Clean 改写，保持调用方语义）。
func sanitizeDelegatePaths(paths []string) ([]string, error) {
	cleaned := make([]string, 0, len(paths))
	for _, p := range paths {
		if p == "" {
			continue
		}
		if strings.HasPrefix(p, "-") {
			return nil, fmt.Errorf("非法路径 %q: 不能以 '-' 开头", p)
		}
		// 路径合法性交上游 pathx.SanitizeRelStrict：拒绝绝对路径、越出工作区
		//（".." 段）、Windows 盘符、UNC 与反斜杠分隔符。此前是
		// filepath.IsAbs + filepath.Clean 的平台相关判定——"..\\.." 形绕过
		// 在 Linux 上按平台解释会漏过。
		if _, ok := pathx.SanitizeRelStrict(p); !ok {
			return nil, fmt.Errorf("非法路径 %q: 必须是工作区内相对路径且不能越出工作区", p)
		}
		cleaned = append(cleaned, p)
	}
	return cleaned, nil
}

// runDelegateCmd 是 delegate 子命令的共用执行器：cmd.Dir=workspace，超时/取消杀进程组。
//
// 返回 (stdout, stderr, err)。stderr 始终被采集，err 非 nil 时调用方应取 stderr 当错误信息。
func (r *Runner) runDelegateCmd(ctx context.Context, binary string, args []string, dir string, cfg Config) (string, string, error) {
	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Dir = dir
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Env = cfg.AppendEnv(childEnv(os.Environ(), cfg.hasExplicitLLMCredential()))

	stdout := newCappedBuffer(maxSubprocessOutput)
	stderr := newCappedBuffer(maxSubprocessOutput)
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	if err := cmd.Start(); err != nil {
		return "", "", &BinaryMissingError{Binary: binary, Err: err}
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

	err := cmd.Wait()
	close(done)
	if err != nil {
		if cmd.Process != nil {
			_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		}
		if ctx.Err() != nil {
			logx.CtxWarnf(ctx, "ocr delegate canceled: %v", err)
		}
		return stdout.String(), stderr.String(), err
	}
	return stdout.String(), stderr.String(), nil
}

// parseDelegatePreview 解析 `ocr delegate preview` 的 markdown 输出。
//
// 输出形如：
//
//	# Files (N reviewable / M total)
//
//	- mode: range
//	- from: HEAD~1
//	- to: HEAD
//	- merge_base: <sha>
//	- total_insertions: 2
//	- total_deletions: 0
//
//	  - `path/file.go` [modified] +2/-0
//
// mode/from/to/merge_base/total_* 通过 `- key: value` 行解析；
// 文件路径通过反引号包裹的路径行提取（“ `path` [status] +N/-M “）。
// 解析失败的行不阻断——返回已提取的部分 + 原始 RawOutput。
func parseDelegatePreview(raw, from, to string) *domain.DelegatePreviewResult {
	out := &domain.DelegatePreviewResult{
		RawOutput: raw,
		From:      from,
		To:        to,
	}
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		// 文件行优先：`- \`path/foo.go\` [modified] +2/-0`
		// 必须在 metadata 解析之前判断，否则 \`path\` 会被当成 metadata key。
		if name, ok := extractDelegateFilePath(line); ok {
			out.Files = append(out.Files, name)
			continue
		}
		// `- key: value` 形式的元数据。
		if strings.HasPrefix(line, "- ") {
			kv := strings.SplitN(strings.TrimPrefix(line, "- "), ":", 2)
			if len(kv) != 2 {
				continue
			}
			k := strings.TrimSpace(kv[0])
			v := strings.TrimSpace(kv[1])
			switch k {
			case "mode":
				out.Mode = v
			case "from":
				out.From = v
			case "to":
				out.To = v
			case "merge_base":
				out.MergeBase = v
			case "total_insertions":
				out.TotalInsert, _ = strconv.Atoi(v)
			case "total_deletions":
				out.TotalDelete, _ = strconv.Atoi(v)
			}
		}
	}
	return out
}

// delegateFileRe 匹配 delegate preview 输出里的 “ `path` [status] “ 行。
// 反引号包裹的路径在前，方括号状态在后；路径可含空格 / unicode。
// 允许前导空白（OCR 输出在元数据块下方缩进 2 空格）。
var delegateFileRe = regexp.MustCompile(`^\s*-\s+` + "`" + `([^` + "`" + `]+)` + "`" + `(?:\s+\[([^\]]+)\])?`)

// extractDelegateFilePath 从一行文本提取反引号内的文件路径。
// 形如 `- \`main.go\` [modified] +2/-0`；不匹配返回 ("", false)。
func extractDelegateFilePath(line string) (string, bool) {
	m := delegateFileRe.FindStringSubmatch(line)
	if m == nil {
		return "", false
	}
	return m[1], true
}
