package ocr

import (
	"errors"
	"fmt"
	"testing"
)

// TestNeedsOperator_AuthFailure 回归测试：凭证失效类失败必须被标记为「需人工介入」。
//
// 这条信号此前只落到 runner 的一行日志里——不看日志就发现不了 token 过期，
// 而它是全仓唯一「机器自己无能为力」的失败类别。
func TestNeedsOperator_AuthFailure(t *testing.T) {
	err := fmt.Errorf("ocr review failed: %w", errors.New("exit status 1: 401 Unauthorized: invalid api key"))
	if !NeedsOperator(err) {
		t.Error("认证失败应标记为 NeedsOperator")
	}
}

// TestNeedsOperator_NilError nil 错误不需要人工介入。
func TestNeedsOperator_NilError(t *testing.T) {
	if NeedsOperator(nil) {
		t.Error("nil error 不应标记 NeedsOperator")
	}
}

// TestNeedsOperator_NonOperatorFailure 普通失败（DNS）不需要人工介入。
func TestNeedsOperator_NonOperatorFailure(t *testing.T) {
	err := fmt.Errorf("ocr review failed: %w", errors.New("dial tcp: lookup ocr.invalid: no such host"))
	if NeedsOperator(err) {
		t.Error("DNS 失败属环境问题，不应标记 NeedsOperator（重试有意义）")
	}
}
