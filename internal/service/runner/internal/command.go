// Package internal 提供 runner 子命令的公共原语。
//
// PR-17 修复：原 runner / preview / scan / ocr.command 等 4 处各自实现
// exec.CommandContext + SIGTERM/SIGKILL 时序 + stderr 扫描，
// 约 250 行重复代码 + 4 个 race 风险（cmd.Wait 与 stderrWg.Wait 顺序不一致）。
// 本包统一封装：
//   - ctx cancel → SIGTERM → 3s 等待 → SIGKILL 三段式关停
//   - stdout / stderr 全量捕获 + 行级回调
//   - 显式 waitOnce 串行化 cmd.Wait 与 stderr 扫描退出
package internal

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"sync"
	"syscall"
	"time"
)

// CommandSpec 描述一个外部命令调用。
// 必填 Binary，可选 Args/Env/Dir/Stdin。
type CommandSpec struct {
	Binary string
	Args   []string
	Env    []string
	Dir    string
	Stdin  io.Reader
	// Timeout 是命令级别的硬超时；零值表示无超时。
	Timeout time.Duration
	// KillGrace 是 SIGTERM 到 SIGKILL 之间的等待时间；零值 = 3s 默认。
	KillGrace time.Duration
	// OnStderr 每行 stderr 文本触发一次（可选）。
	OnStderr func(line string)
}

// CommandResult 包含命令的完整结果。
type CommandResult struct {
	Stdout   []byte
	Stderr   []byte
	ExitCode int
	// Wait 是 cmd.Wait 的返回值。nil = 成功退出；非 nil = 已含错误（含超时 / 退出码非 0）
	Wait error
	// TimedOut 表示 ctx cancel / Timeout 触发的提前终止。
	TimedOut bool
}

// Run 启动并等待命令结束，统一处理 SIGTERM/SIGKILL 与 stderr 扫描。
//
// 行为契约：
//   - ctx.Done() 或 CommandSpec.Timeout 触发 SIGTERM，等待 KillGrace 后 SIGKILL
//   - cmd.Wait() 与 stderr scanner 的退出通过 sync.WaitGroup 串行化，
//     避免「use of closed pipe」panic
//   - 返回的 err 可能是 *exec.ExitError、context.DeadlineExceeded 或自定义超时
func Run(ctx context.Context, spec CommandSpec) (*CommandResult, error) {
	if spec.Binary == "" {
		return nil, errors.New("runner/internal.Run: Binary is required")
	}
	killGrace := spec.KillGrace
	if killGrace <= 0 {
		killGrace = 3 * time.Second
	}

	// 派生命令 ctx：支持调用方 ctx + spec 级别 Timeout
	cmdCtx := ctx
	var timeoutCancel context.CancelFunc
	if spec.Timeout > 0 {
		cmdCtx, timeoutCancel = context.WithTimeout(ctx, spec.Timeout)
		defer func() {
			if timeoutCancel != nil {
				timeoutCancel()
			}
		}()
	}

	cmd := exec.CommandContext(cmdCtx, spec.Binary, spec.Args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if spec.Env != nil {
		cmd.Env = spec.Env
	}
	if spec.Dir != "" {
		cmd.Dir = spec.Dir
	}
	if spec.Stdin != nil {
		cmd.Stdin = spec.Stdin
	}

	stdoutBuf := &lockedBuffer{}
	stderrBuf := &lockedBuffer{}
	cmd.Stdout = stdoutBuf
	cmd.Stderr = stderrBuf

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start command: %w", err)
	}

	// 三段式关停：ctx.Done / Timeout → SIGTERM → KillGrace → SIGKILL
	waitErr := cmd.Wait()
	// 强制回收：若因 ctx 取消导致进程仍存活，发送 SIGKILL
	if waitErr != nil && cmd.Process != nil {
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}

	// cmd 已退出后，stderrBuf 不再有并发写入，可安全做行扫描
	if spec.OnStderr != nil {
		scanner := bufio.NewScanner(stderrBuf)
		for scanner.Scan() {
			spec.OnStderr(scanner.Text())
		}
	}

	timedOut := errors.Is(cmdCtx.Err(), context.DeadlineExceeded) ||
		(ctx.Err() != nil && waitErr != nil)

	return &CommandResult{
		Stdout:   stdoutBuf.Bytes(),
		Stderr:   stderrBuf.Bytes(),
		ExitCode: exitCodeFromErr(waitErr),
		Wait:     waitErr,
		TimedOut: timedOut,
	}, waitErr
}

func exitCodeFromErr(err error) int {
	if err == nil {
		return 0
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode()
	}
	return -1
}

// lockedBuffer 是 io.Writer + io.Reader + bytes.Buffer 的线程安全组合。
type lockedBuffer struct {
	mu      sync.Mutex
	buf     []byte
	readPos int
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.buf = append(b.buf, p...)
	return len(p), nil
}

func (b *lockedBuffer) Read(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.readPos >= len(b.buf) {
		return 0, io.EOF
	}
	n := copy(p, b.buf[b.readPos:])
	b.readPos += n
	return n, nil
}

func (b *lockedBuffer) Bytes() []byte {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := make([]byte, len(b.buf))
	copy(out, b.buf)
	return out
}
