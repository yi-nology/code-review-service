// failure.go：OCR 子进程失败的**分类判定**——判据表来自底座 agent-harness/toolfail。
//
// 此前本文件把「不可重试」压成一个 PermanentError：DNS 解析失败、认证失败、
// 连接被拒统统归为同一类，处置只有「不重试」一种。
//
// 但这三类的处置并不相同：
//   - DNS 解析失败 / 连接被拒：环境问题，重试无意义（但不需要人介入）
//   - 认证失败 / 权限不足：**要人介入**（token 过期、权限没配），重试同样无意义
//   - 限流：值得重试，且是唯一能证明「服务端尚未受理本次请求」的类别
//
// 把它们压成一类，后果是认证失败只留下一条「ocr 评审失败」的日志，
// 运维看不出「这是配置问题，等人改 token」——而这两者的处置动作完全不同。
//
// 现在判定下沉到底座：toolfail 的类别 → 策略表（可重试/退避/需人工/重发安全）
// 由底座单源持有，宿主不再另立一张判据表。
//
// 底座只判定不处置：它给出「这次失败属于哪一类、值不值得重试」，
// **重试调度仍在宿主**（runner.go 的退避环），人工介入的通知也仍归宿主。
package ocr

import (
	"context"
	"errors"
	"strings"

	"github.com/yi-nology/agent-harness/toolfail"
)

// classifyOCRFailure 把 OCR 子进程失败映射到底座类别。
//
// 映射口径（宿主知识：OCR 的失败长什么样）：
//   - 超时 → transient（值得重试；但**不能断言重发安全**——服务端可能已执行）
//   - DNS 解析失败 / 连接被拒 → permanent（环境问题，重试只会再等一个 timeout）
//   - 认证失败 / 权限不足 → authentication（要人介入：token 过期或权限没配）
//   - 限流 → rate_limited（值得重试，且重发安全）
//   - 其余非零退出 → transient（退出原因未知时给可重试的缺省，
//     而不是不可重试——不可重试的缺省会让一次瞬时抖动直接判死整个评审）
//
// 最后一条是刻意的：底座的保守缺省是「不可分类 = 不可重试」，
// 但那是**通用退避循环**的缺省（盲目重试放大失败）。本宿主有明确的退出码与
// stderr 线索，且重试成本有界（最多 1 次 + 指数退避），
// 故这里显式归类为 transient 而非落到 unknown。
func classifyOCRFailure(err error, stderr string) toolfail.Class {
	if err == nil {
		return toolfail.ClassUnknown
	}

	// 已有底座分类（嵌套调用已分类过）：直接沿用，不重复判定。
	if f, ok := toolfail.As(err); ok && f != nil && f.Class != "" {
		return f.Class
	}

	lower := strings.ToLower(stderr)
	if errText := strings.ToLower(err.Error()); errText != "" {
		lower = lower + " " + errText
	}

	switch {
	case IsTimeout(err):
		return toolfail.ClassTransient

	case containsAny(lower, authMessagePatterns):
		// 认证/权限：不可重试，且**需要人工介入**（底座策略表里
		// ClassAuthentication 的 NeedsOperator 为真）。
		return toolfail.ClassAuthentication

	case containsAny(lower, rateLimitMessagePatterns):
		// 限流：底座唯一断言「重发安全」的类别（请求尚未进入业务逻辑）。
		return toolfail.ClassRateLimited

	case containsAny(lower, permanentMessagePatterns):
		// DNS 解析失败 / 连接被拒：环境问题，重试无意义。
		return toolfail.ClassPermanent

	default:
		return toolfail.ClassTransient
	}
}

// stderrOf 从错误链上取出 stderr 文本。
//
// 分类需要看子进程输出（DNS 失败、401、429 都只在 stderr 里），
// 而错误值本身可能已被多层包装（runner → 上层）。
func stderrOf(err error) string {
	var e *ExitError
	if errors.As(err, &e) && e != nil {
		return e.Stderr
	}
	var p *PermanentError
	if errors.As(err, &p) && p != nil {
		return p.Stderr
	}
	return ""
}

// containsAny 判断文本是否包含任一特征串。
//
// **双方都小写化**：模式表里混着大写（ENOTFOUND / ECONNREFUSED 这些是
// Node.js 的错误码原文）与小写措辞，若只把输入小写化，大写模式永远匹配不上。
//
// 这不是假设——重构前就踩过这个坑：输入被小写化、模式表原样匹配，
// 于是 "ENOTFOUND" 与 "ECONNREFUSED" 两条**从未生效**，
// Node.js 的 DNS 失败被判成可重试，白白再等一个 timeout。
func containsAny(lower string, patterns []string) bool {
	for _, pat := range patterns {
		if pat != "" && strings.Contains(lower, strings.ToLower(pat)) {
			return true
		}
	}
	return false
}

// ocrFailure 把一个错误包装成底座失败值。
//
// 包装后整条链路都能被 toolfail.Retryable / toolfail.As 直接捡走，
// 宿主与上游对「这次失败值不值得重试」不再各判一套。
func ocrFailure(err error, stderr string) *toolfail.Failure {
	if err == nil {
		return nil
	}
	return toolfail.ClassifyAs(classifyOCRFailure(err, stderr), err)
}

// ocrRetryable 判断一次 OCR 失败是否值得原样重试。
//
// 这是 runner 重试环的唯一判据入口——不再问「这是不是 PermanentError」，
// 而是问底座「这次失败重试有没有收益」。
func ocrRetryable(err error, stderr string) bool {
	if err == nil {
		return false
	}
	// 上下文已取消：重试无意义（外层已经结束）。
	if errors.Is(err, context.Canceled) {
		return false
	}
	return toolfail.Retryable(ocrFailure(err, stderr))
}

// ocrNeedsOperator 判断一次失败是否需要人工介入。
//
// 认证失败/权限不足必须让人看到：等它自愈不会发生，
// 而它的症状（评审一直失败）与「模型端点抖动」在监控上无法区分。
func ocrNeedsOperator(err error, stderr string) bool {
	if err == nil {
		return false
	}
	f := ocrFailure(err, stderr)
	if f == nil {
		return false
	}
	return toolfail.PolicyOf(f.Class).NeedsOperator
}

// authMessagePatterns 匹配认证/权限类失败的 stderr 特征。
//
// 与永久性失败分开：认证失败要人介入（换 token、配权限），
// 而 DNS 失败不用（等网络恢复）。两者都不可重试，但处置动作不同。
//
// 刻意**不用裸状态码**（"401"/"403"）：stderr 里出现的数字未必是状态码
// （行号、字节数、重试次数都可能撞上），而误判成认证失败会让一次
// 普通失败被标成「要人介入」，运维被告警淹没后真信号反而被忽略。
// 只认带上下文的措辞——状态码总是跟在 "HTTP"/"status" 之后。
var authMessagePatterns = []string{
	"401 unauthorized",
	"403 forbidden",
	"unauthorized",
	"forbidden",
	"invalid token",
	"invalid api key",
	"unauthorized access",
	"authentication failed",
	"permission denied",
	"api key",
}

// rateLimitMessagePatterns 匹配限流类失败的 stderr 特征。
//
// 限流与其它失败相反：值得重试，且是唯一能证明「服务端未受理本次请求」的类别，
// 原样重发安全。
//
// 同样**不用裸 "429"**（理由同 authMessagePatterns：裸数字会撞上行号等无关内容，
// 把普通失败误判成限流会导致一次失败被白白重发）。
var rateLimitMessagePatterns = []string{
	"429 too many requests",
	"rate limit",
	"rate_limit",
	"ratelimit",
	"too many requests",
	"quota exceeded",
	"overloaded",
}
