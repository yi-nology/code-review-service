package runner

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

// 本文件定义 OCR 评审失败的分类错误类型。runOnce 用 %w/具体类型包裹原始错误，
// 上层 executor.recordFailure 通过 errors.As 判断类型，在 review history 里标记
// error_type，让运维能针对性响应：
//   - binary_missing：二进制缺失/不可执行 → 修 OCR 安装或 Binary 配置路径
//   - timeout：评审超时 → 调 TimeoutMinutes 或查 LLM 延迟
//   - exit：子进程非零退出（含 LLM/DNS/解析错误）→ 看 stderr 明细

// BinaryMissingError 表示 OCR 二进制不存在或不可执行（cmd.Start 失败）。
type BinaryMissingError struct {
	Binary string
	Err    error
}

func (e *BinaryMissingError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("ocr 二进制 %q 不可用: %v", e.Binary, e.Err)
	}
	return fmt.Sprintf("ocr 二进制 %q 不可用", e.Binary)
}

func (e *BinaryMissingError) Unwrap() error { return e.Err }

// TimeoutError 表示 OCR 评审超过 context deadline。
type TimeoutError struct {
	Timeout time.Duration
}

func (e *TimeoutError) Error() string {
	return fmt.Sprintf("ocr 评审超时（上限 %s）", e.Timeout)
}

// ExitError 表示 OCR 子进程非零退出，Stderr 含根因（如 LLM DNS 失败 / 5xx / 输出解析失败）。
type ExitError struct {
	Stderr string
	Err    error
}

func (e *ExitError) Error() string {
	msg := strings.TrimSpace(e.Stderr)
	if msg == "" && e.Err != nil {
		msg = e.Err.Error()
	}
	if msg == "" {
		return "ocr 评审失败"
	}
	return fmt.Sprintf("ocr 评审失败: %s", msg)
}

func (e *ExitError) Unwrap() error { return e.Err }

// classifyExitError 把 cmd.Wait 的失败归类为 TimeoutError 或 ExitError。
// OCR 子进程被 ctx 超时杀掉时，ctx.Err() 为 DeadlineExceeded；
// 其它非零退出（LLM 错误、输出解析失败）归为 ExitError，保留 stderr 供排障。
func classifyExitError(ctx context.Context, runErr error, timeout time.Duration, stderr string) error {
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return &TimeoutError{Timeout: timeout}
	}
	return &ExitError{Stderr: stderr, Err: runErr}
}

// IsBinaryMissing / IsTimeout / IsExit 供上层用 errors.As 判断（也便于测试）。
func IsBinaryMissing(err error) bool {
	var e *BinaryMissingError
	return errors.As(err, &e)
}

func IsTimeout(err error) bool {
	var e *TimeoutError
	return errors.As(err, &e)
}

func IsExit(err error) bool {
	var e *ExitError
	return errors.As(err, &e)
}
