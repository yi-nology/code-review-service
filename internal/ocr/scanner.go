package ocr

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"

	"github.com/yi-nology/code-review-service/internal/service/review/domain"

	"github.com/oekit/concurrency/async"
)

// Preview 实现 domain.PreviewRunner（吸收 runner/preview.go）。
//
// 调 "ocr scan --preview" 子进程，返回将参与评审的文件列表。
func (r *Runner) Preview(ctx context.Context, workspace domain.Workspace) (*domain.PreviewResult, error) {
	if strings.TrimSpace(workspace.Dir) == "" {
		return nil, fmt.Errorf("需要已检出的工作区")
	}

	args := []string{
		"scan", "--preview",
		"--repo", workspace.Dir,
		"--format", "json",
	}
	toolsArgs, cleanupTools, err := appendToolsArgs(args, r.cfg)
	if err != nil {
		return nil, err
	}
	args = toolsArgs
	if cleanupTools != nil {
		defer cleanupTools()
	}

	cmd := exec.CommandContext(ctx, r.cfg.Binary, args...)
	cmd.Dir = workspace.Dir
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Env = r.cfg.AppendEnv(os.Environ())

	stdout := newCappedBuffer(maxSubprocessOutput)
	stderr := newCappedBuffer(maxSubprocessOutput)
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	if err := cmd.Start(); err != nil {
		return nil, &BinaryMissingError{Binary: r.cfg.Binary, Err: err}
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

	var out domain.PreviewResult
	if err := json.Unmarshal(stdout.Bytes(), &out); err != nil {
		return nil, fmt.Errorf("parse ocr preview output: %w", err)
	}
	return &out, nil
}

// Scan 实现 domain.ScanRunner（吸收 runner/scan.go）。
//
// 调 "ocr scan" 子进程（不带 --preview），返回全量扫描结果。
// 支持 batch / paths / exclude / background / tools 等参数。
func (r *Runner) Scan(ctx context.Context, workspace domain.Workspace, req domain.ScanRequest) (*domain.ScanResult, error) {
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
	if req.Background != "" {
		args = append(args, "--background", req.Background)
	}
	toolsArgs, cleanupTools, err := appendToolsArgs(args, r.cfg)
	if err != nil {
		return nil, err
	}
	args = toolsArgs
	if cleanupTools != nil {
		defer cleanupTools()
	}

	cmd := exec.CommandContext(ctx, r.cfg.Binary, args...)
	cmd.Dir = workspace.Dir
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Env = r.cfg.AppendEnv(os.Environ())

	stdout := newCappedBuffer(maxSubprocessOutput)
	stderr := newCappedBuffer(maxSubprocessOutput)
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	if err := cmd.Start(); err != nil {
		return nil, &BinaryMissingError{Binary: r.cfg.Binary, Err: err}
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

	var out domain.ScanResult
	if err := json.Unmarshal(stdout.Bytes(), &out); err != nil {
		return nil, fmt.Errorf("parse ocr scan output: %w", err)
	}
	return &out, nil
}
