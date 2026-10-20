package ocr

import (
	"errors"
	"testing"

	"github.com/yi-nology/agent-harness/toolfail"
)

// TestClassifyOCRFailureDNSIsPermanent DNS 解析失败属环境类问题：
// 不可重试，但**不需要人工介入**（等网络恢复即可）。
func TestClassifyOCRFailureDNSIsPermanent(t *testing.T) {
	err := &ExitError{Stderr: "getaddrinfo ENOTFOUND llm.internal", Err: errors.New("exit status 1")}

	if got := classifyOCRFailure(err, err.Stderr); got != toolfail.ClassPermanent {
		t.Fatalf("DNS 失败应归 permanent，实际 %s", got)
	}
	if ocrRetryable(err, err.Stderr) {
		t.Fatal("DNS 失败不应重试")
	}
	if ocrNeedsOperator(err, err.Stderr) {
		t.Fatal("DNS 失败不应需要人工介入（等网络恢复即可）")
	}
}

// TestClassifyOCRFailureAuthNeedsOperator 钉住本次重构的关键语义：
//
// 认证失败与 DNS 失败**都不可重试**，但处置动作不同——
// 认证失败要人介入（token 过期、权限没配），等它自愈不会发生。
//
// 此前两者被压成同一个 PermanentError，运维看不出「这是配置问题」，
// 症状（评审一直失败）与「模型端点抖动」在监控上无法区分。
func TestClassifyOCRFailureAuthNeedsOperator(t *testing.T) {
	cases := []string{
		"HTTP 401 Unauthorized",
		"Error: invalid api key",
		"403 forbidden",
		"authentication failed for model endpoint",
	}

	for _, stderr := range cases {
		err := &ExitError{Stderr: stderr, Err: errors.New("exit status 1")}

		got := classifyOCRFailure(err, stderr)
		if got != toolfail.ClassAuthentication {
			t.Fatalf("stderr=%q 应归 authentication，实际 %s", stderr, got)
		}
		if ocrRetryable(err, stderr) {
			t.Fatalf("stderr=%q 认证失败不应重试", stderr)
		}
		if !ocrNeedsOperator(err, stderr) {
			t.Fatalf("stderr=%q 认证失败必须标记为需要人工介入", stderr)
		}
	}
}

// TestClassifyOCRFailureRateLimitIsRetryable 限流值得重试，
// 且是唯一能证明「服务端尚未受理本次请求」的类别（重发安全）。
func TestClassifyOCRFailureRateLimitIsRetryable(t *testing.T) {
	cases := []string{
		"429 Too Many Requests",
		"rate limit exceeded for model endpoint",
		"upstream overloaded, retry later",
	}

	for _, stderr := range cases {
		err := &ExitError{Stderr: stderr, Err: errors.New("exit status 1")}

		got := classifyOCRFailure(err, stderr)
		if got != toolfail.ClassRateLimited {
			t.Fatalf("stderr=%q 应归 rate_limited，实际 %s", stderr, got)
		}
		if !ocrRetryable(err, stderr) {
			t.Fatalf("stderr=%q 限流应可重试", stderr)
		}
	}
}

// TestClassifyOCRFailureTimeoutIsTransient 超时值得重试，
// 但**不能断言重发安全**——服务端可能已执行。
func TestClassifyOCRFailureTimeoutIsTransient(t *testing.T) {
	err := &TimeoutError{Timeout: 300}

	if got := classifyOCRFailure(err, ""); got != toolfail.ClassTransient {
		t.Fatalf("超时应归 transient，实际 %s", got)
	}
	if !ocrRetryable(err, "") {
		t.Fatal("超时值得重试")
	}

	f := ocrFailure(err, "")
	if f == nil {
		t.Fatal("ocrFailure 应返回失败值")
	}
	if f.SafeToResubmit() {
		t.Fatal("超时不得断言重发安全——服务端可能已执行，原样重发会重复施加副作用")
	}
}

// TestClassifyOCRFailureUnknownIsTransient 退出原因未知时归 transient。
//
// 这与底座的保守缺省（不可分类 = 不可重试）不同，理由见
// classifyOCRFailure 的注释：宿主有 stderr 线索且重试成本有界。
// 归不可重试会让一次瞬时抖动直接判死整个评审。
func TestClassifyOCRFailureUnknownIsTransient(t *testing.T) {
	err := &ExitError{Stderr: "some unexplained failure", Err: errors.New("exit status 1")}

	if got := classifyOCRFailure(err, err.Stderr); got != toolfail.ClassTransient {
		t.Fatalf("未知退出原因应归 transient，实际 %s", got)
	}
}

// TestClassifyOCRFailureKeepsExistingClass 已有底座分类时不得重复判定——
// 嵌套调用已分类过，重复判定会把更具体的结论洗掉。
func TestClassifyOCRFailureKeepsExistingClass(t *testing.T) {
	existing := toolfail.ClassifyAs(toolfail.ClassInvalidArgument, errors.New("bad param"))

	if got := classifyOCRFailure(existing, "no such host"); got != toolfail.ClassInvalidArgument {
		t.Fatalf("已有分类应保留，实际 %s", got)
	}
}

// TestOCRFailureGuidanceIsConsumable 底座的处置指引必须有消费方——
// 判据建了没人读等于没建。
func TestOCRFailureGuidanceIsConsumable(t *testing.T) {
	err := &ExitError{Stderr: "401 Unauthorized", Err: errors.New("exit status 1")}

	f := ocrFailure(err, err.Stderr)
	if f == nil {
		t.Fatal("ocrFailure 不应返回 nil")
	}
	guidance := f.Guidance()
	if guidance == "" {
		t.Fatal("处置指引不应为空")
	}
}
