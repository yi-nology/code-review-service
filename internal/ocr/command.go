package ocr

import (
	"context"
	"time"

	ocrinternal "github.com/yi-nology/code-review-service/internal/ocr/internal"
)

// Public aliases to internal/ocr/internal（让 ocr.Run / ocr.CommandSpec 等可被外部使用）。
// 这些函数已迁移到 ocr 包顶层，不再留在 internal/ 子包。

// CommandSpec 描述一次外部命令调用。
type CommandSpec = ocrinternal.CommandSpec

// CommandResult 包含命令的完整结果。
type CommandResult = ocrinternal.CommandResult

// InternalRun 是统一的 exec 入口。
func InternalRun(ctx context.Context, spec CommandSpec) (*CommandResult, error) {
	return ocrinternal.Run(ctx, spec)
}

// Run 是简化的 exec 调用（直接传 binary + args + timeout）。
//
// 注意：Timeout 为 0，子进程仅受传入 ctx 控制。对元信息类调用（如 --version），
// 调用方往往不传可超时 ctx（HTTP handler 的 ctx 只在客户端断开时取消），
// 此时若子进程挂起会无限阻塞 goroutine。这类调用请改用 RunWithTimeout。
func Run(ctx context.Context, binary string, args ...string) (stdout []byte, stderr []byte, err error) {
	res, err := ocrinternal.Run(ctx, CommandSpec{
		Binary:  binary,
		Args:    args,
		Timeout: 0,
	})
	if res == nil {
		return nil, nil, err
	}
	return res.Stdout, res.Stderr, err
}

// RunWithTimeout 是 Run 的带超时变体：无论 ctx 是否可超时，子进程最多运行 timeout。
// 用于 ocr --version 这类元信息探测——避免客户端 30s 超时后 goroutine 仍被挂起的子进程阻塞。
// 返回的 err 在超时时为 context.DeadlineExceeded。
func RunWithTimeout(ctx context.Context, timeout time.Duration, binary string, args ...string) (stdout []byte, stderr []byte, err error) {
	res, runErr := ocrinternal.Run(ctx, CommandSpec{
		Binary:  binary,
		Args:    args,
		Timeout: timeout,
	})
	if res == nil {
		return nil, nil, runErr
	}
	return res.Stdout, res.Stderr, runErr
}
