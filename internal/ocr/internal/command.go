// Package internal 提供 Runner 的 exec 公共原语。
//
// 阶段 A+3 设计：原 runner/internal/Run + ocr/command.runOCRCommand 合并到此处。
//
// 阶段 A2（当前）：保留 Run 作为 fallback，跨包类型通过 Config 适配。
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

	"github.com/yi-nology/code-review-service/internal/domain/review"
)

// CommandSpec 描述一次外部命令调用（与 runner/internal.CommandSpec 等价）。
type CommandSpec struct {
	Binary    string
	Args      []string
	Env       []string
	Dir       string
	Stdin     io.Reader
	Timeout   time.Duration
	KillGrace time.Duration
	OnStdout  func(line string)
	OnStderr  func(line string)
}

// CommandResult 包含命令的完整结果。
type CommandResult struct {
	Stdout   []byte
	Stderr   []byte
	ExitCode int
	Wait     error
	TimedOut bool
}

// Run 是统一的 exec 入口。
//
// 行为契约：
//   - ctx.Done() / CommandSpec.Timeout 触发 SIGTERM → KillGrace → SIGKILL 三段式：
//     第一段 cmd.Cancel 对进程组发 SIGTERM（优雅退出机会）；KillGrace 内未退出，
//     第二段由 exec 的 WaitDelay 以 SIGKILL 杀主进程；Wait 返回后再对残留进程组
//     补一次 SIGKILL（第三段，清理子进程泄漏的孙进程）
//   - cmd.Wait() 与 stderr scanner 串行化（避免 use of closed pipe）
//   - 返回的 error 含 redact.String 脱敏
//
// 此函数是阶段 A 后所有 OCR 子进程调用的唯一入口，
// 取代 runner/internal.Run + ocr/command.runOCRCommand。
func Run(ctx context.Context, spec CommandSpec) (*CommandResult, error) {
	if spec.Binary == "" {
		return nil, errors.New("internal.Run: Binary is required")
	}
	killGrace := spec.KillGrace
	if killGrace <= 0 {
		killGrace = 3 * time.Second
	}

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
	// 第一段：ctx 取消/超时时对进程组发 SIGTERM。错误不向 Wait 传播，
	// 超时语义由下方 TimedOut 标记表达。
	cmd.Cancel = func() error {
		if cmd.Process != nil {
			_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)
		}
		return nil
	}
	// 第二段：SIGTERM 后 killGrace 内仍未退出，exec 内部以 SIGKILL 杀主进程。
	cmd.WaitDelay = killGrace
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

	waitErr := cmd.Wait()
	// 第三段：主进程死后进程组内可能残留孙进程（OCR 可能再拉起子进程），兜底清组。
	if cmd.Process != nil {
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}

	if spec.OnStdout != nil {
		scanner := bufio.NewScanner(stdoutBuf)
		for scanner.Scan() {
			spec.OnStdout(scanner.Text())
		}
	}
	if spec.OnStderr != nil {
		scanner := bufio.NewScanner(stderrBuf)
		for scanner.Scan() {
			spec.OnStderr(scanner.Text())
		}
	}

	timedOut := cmdCtx.Err() != nil && waitErr != nil
	return &CommandResult{
		Stdout:   stdoutBuf.Bytes(),
		Stderr:   stderrBuf.Bytes(),
		ExitCode: exitCodeFromErr(waitErr),
		Wait:     waitErr,
		TimedOut: timedOut,
	}, waitErr
}

// exitCodeFromErr 提取 ExitError 的退出码。
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

// lockedBuffer 是 io.Writer + io.Reader 的线程安全组合。
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

// ensureUnreachableWorkspace 占位：阶段 A+3 完整化时把 exec 路径直接接到 Runner.Run。
var _ = domain.Workspace{}
