package ocr

import (
	"testing"
	"unicode/utf8"
)

// TestTruncateMiddle_UTF8Safe 回归测试：truncateMiddle 必须按 rune 切。
//
// 此前实现按字节切（s[:half]），中文入参会腰斩多字节字符，产出非法 UTF-8。
// 调用点是 LLM 降级原因——中文占比很高，这条路径不是理论风险。
func TestTruncateMiddle_UTF8Safe(t *testing.T) {
	cases := []struct {
		name   string
		in     string
		maxLen int
	}{
		{"纯中文", "评审失败：上下文超限，建议拆分变更后重试", 10},
		{"中英混合", "rate limit exceeded：请求过于频繁，请稍后重试", 10},
		{"emoji", "🚨 review failed 🚨 review failed 🚨 review failed", 10},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := truncateMiddle(tc.in, tc.maxLen)
			if !utf8.ValidString(got) {
				t.Errorf("输出非法 UTF-8: %q", got)
			}
			if got == tc.in {
				t.Errorf("应发生截断，却原样返回: %q", got)
			}
		})
	}
}

// TestTruncateMiddle_ShortInput 验证未超预算时原样返回。
func TestTruncateMiddle_ShortInput(t *testing.T) {
	if got := truncateMiddle("短文本", 100); got != "短文本" {
		t.Errorf("未超预算应原样返回, got %q", got)
	}
}

// TestTruncateMiddle_TinyBudget 验证极小预算下不产出「省略号 + 全文」这种
// 比原文还长的结果。
func TestTruncateMiddle_TinyBudget(t *testing.T) {
	got := truncateMiddle("这是一段很长的中文文本", 1)
	if len([]rune(got)) > 1 {
		t.Errorf("maxLen=1 时应退化为前缀截断, got %q", got)
	}
}
