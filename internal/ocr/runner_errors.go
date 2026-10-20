package ocr

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/yi-nology/code-review-service/internal/service/review/domain"

	"github.com/oekit/logx"
	"github.com/yi-nology/agent-harness/toolfail"
)

// runner_errors.go：子进程输出采集（cappedBuffer）与错误分类（超时/退出/永久性/二进制缺失）。
// BinaryMissingError 表示 OCR 二进制不存在或不可执行（cmd.Start 失败）。
type BinaryMissingError struct {
	Binary string
	Err    error
}

// cappedBuffer 是带容量上限的 bytes.Buffer：超过 cap 后丢弃后续写入并记录一次告警，
// 作为 exec.Cmd 的 Stdout/Stdout 捕获器，限制输出内存占用。
type cappedBuffer struct {
	buf    bytes.Buffer
	cap    int
	capped bool
}

func newCappedBuffer(capBytes int) *cappedBuffer {
	return &cappedBuffer{cap: capBytes}
}

func (b *cappedBuffer) Write(p []byte) (int, error) {
	if b.capped {
		return len(p), nil
	}
	room := b.cap - b.buf.Len()
	if len(p) >= room {
		if room > 0 {
			b.buf.Write(p[:room])
		}
		b.capped = true
		logx.Warnf("ocr subprocess output exceeded %d bytes cap, further output discarded", b.cap)
		return len(p), nil
	}
	return b.buf.Write(p)
}

func (b *cappedBuffer) Bytes() []byte { return b.buf.Bytes() }

func (b *cappedBuffer) String() string { return b.buf.String() }

// classifyExitError 把 cmd.Wait 的失败归类为 TimeoutError、PermanentError 或 ExitError。
//
// 对外的错误类型**保持不变**（调用方按 errors.As 判别，这是既有契约），
// 但内部分类不再靠字符串匹配自己判「不可重试」——
// 类别判定下沉到 harness/toolfail（见 failure.go）：
// 认证失败与 DNS 失败虽然都不可重试，前者**需要人工介入**，
// 压成一类会让运维看不出「这是配置问题」。
func classifyExitError(ctx context.Context, runErr error, timeout time.Duration, stderr string) error {
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return &TimeoutError{Timeout: timeout}
	}
	combined := stderr
	if runErr != nil {
		combined = combined + " " + runErr.Error()
	}
	switch classifyOCRFailure(runErr, combined) {
	case toolfail.ClassPermanent, toolfail.ClassAuthentication, toolfail.ClassNotFound:
		return &PermanentError{Stderr: stderr, Err: runErr}
	default:
		return &ExitError{Stderr: stderr, Err: runErr}
	}
}

// TimeoutError 表示 OCR 评审超过 context deadline。
type TimeoutError struct {
	Timeout time.Duration
}

// ExitError 表示 OCR 子进程非零退出，Stderr 含根因。
type ExitError struct {
	Stderr string
	Err    error
}

func (e *BinaryMissingError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("ocr 二进制 %q 不可用: %v", e.Binary, e.Err)
	}
	return fmt.Sprintf("ocr 二进制 %q 不可用", e.Binary)
}

func (e *TimeoutError) Error() string {
	return fmt.Sprintf("ocr 评审超时（上限 %s）", e.Timeout)
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

func (e *PermanentError) Error() string {
	msg := strings.TrimSpace(e.Stderr)
	if msg == "" && e.Err != nil {
		msg = e.Err.Error()
	}
	if msg == "" {
		return "ocr 评审遇到不可重试错误"
	}
	return fmt.Sprintf("ocr 评审遇到不可重试错误: %s", msg)
}

func (e *BinaryMissingError) Unwrap() error { return e.Err }

func (e *ExitError) Unwrap() error { return e.Err }

func (e *PermanentError) Unwrap() error { return e.Err }

// PermanentError 表示不可重试的错误（如 DNS 解析失败、认证失败）。
// 重试只会双倍超时，Run 循环遇到此类错误立即终止。
type PermanentError struct {
	Stderr string
	Err    error
}

// IsPermanentError 供 errors.As 判断是否为 PermanentError。
func IsPermanentError(err error) bool {
	var e *PermanentError
	return errors.As(err, &e)
}

// IsBinaryMissing / IsTimeout / IsExit / IsPermanent 供上层用 errors.As 判断。
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

// IsPermanent 判断错误是否不可重试（含 PermanentError 及其包装）。
func IsPermanent(err error) bool {
	return IsPermanentError(err)
}

// Runner 是 open-code-review 子进程调用入口（阶段 A12 完整化）。
//
// 实现了 domain.Runner / domain.PreviewRunner / domain.ScanRunner 三接口。
//
// 行为：
//   - Run: 调 "ocr review --repo --from --to ..." 子进程，解析 JSON 输出
//   - Preview: 调 "ocr scan --preview ..." 子进程
//   - Scan: 调 "ocr scan ..." 子进程，支持 batch / paths / exclude
//
// 阶段 A12：完整吸收 runner/runner.go + runner/preview.go + runner/scan.go 业务逻辑。
// runnerRefreshInterval 是后台刷新 baseCfg 的间隔。
// OCR 设置变更频率极低（人工操作），30s 足够；PUT handler 会通过 NotifyRefresh 立即触发。
const runnerRefreshInterval = 30 * time.Second

// Compile-time assertions
var (
	_ domain.Runner        = (*Runner)(nil)
	_ domain.PreviewRunner = (*Runner)(nil)
	_ domain.ScanRunner    = (*Runner)(nil)
	_ domain.Delegator     = (*Runner)(nil)
)

// defaultOCRMaxRetries 默认重试 1 次。
// OCR 评审是 LLM 调用，LLM 端点偶发 5xx / 超时很常见，0 重试会让网关服务过于脆弱。
// 1 次重试 + 指数退避在「多救一次瞬时抖动」与「不显著拉长评审耗时」间取平衡。
// BinaryMissingError（二进制根本不存在）和 ctx 取消不重试——重试无意义。
const defaultOCRMaxRetries = 1

// retryBaseBackoff 是首次重试前的指数退避基准。
const retryBaseBackoff = 2 * time.Second

// maxSubprocessOutput 限制单个 OCR 子进程 stdout/stderr 的内存上限，
// 防止健谈/失控的二进制把内存撑爆（OOM）。正常 JSON 输出远小于此。
const maxSubprocessOutput = 64 << 20 // 64 MB

// permanentMessagePatterns 匹配**环境类**不可恢复错误的特征（DNS 解析失败、连接被拒）。
//
// 这些错误重试无意义：ocr 子进程访问 LLM 端点时域名解析不到，
// 短时间内不会自愈，重试只会双倍超时。
//
// 注意它**不再**覆盖认证失败——认证失败另有归属（见 failure.go 的
// authMessagePatterns），因为两者处置动作不同：前者等环境恢复，
// 后者要人换 token / 配权限。
var permanentMessagePatterns = []string{
	"no such host",          // DNS NXDOMAIN（getaddrinfo 标准措辞）
	"lookup",                // DNS 解析失败前缀（如 "lookup xxx on ..."）
	"ENOTFOUND",             // Node.js DNS 错误码（ocr 可能用 node）
	"ECONNREFUSED",          // 连接被拒（端口未开，非瞬时抖动）
	"nodename nor servname", // macOS DNS 失败措辞
	"server misbehaving",    // DNS SERVFAIL
}
