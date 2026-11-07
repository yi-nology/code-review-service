package finding

import (
	"strings"
	"testing"
)

// line=0 findings 的统计分母显式带出（issue #2）：摘要必须让读者看到
// 「有 N 条没法锚定行号」，否则行内评论变少的原因不可分辨。
func TestFormatReviewReport_UnlocatableShown(t *testing.T) {
	findings := []FormatFinding{
		{Severity: "major", Title: "a", Line: 12},
		{Severity: "minor", Title: "b", Line: 0},
		{Severity: "info", Title: "c", Line: 0},
	}
	out := FormatReviewReport("<!-- marker -->", "summary", findings, 10, false)
	if !strings.Contains(out, "**Unlocatable findings:** 2") {
		t.Fatalf("摘要未带出未锚定计数:\n%s", out)
	}
	if !strings.Contains(out, "Findings:** 0 critical, 1 major") {
		t.Fatalf("severity 计数被破坏:\n%s", out)
	}
}

func TestFormatReviewReport_AllLocatableNoNoise(t *testing.T) {
	findings := []FormatFinding{
		{Severity: "major", Title: "a", Line: 12},
	}
	out := FormatReviewReport("<!-- marker -->", "summary", findings, 10, false)
	if strings.Contains(out, "Unlocatable") {
		t.Fatalf("全锚定时不应出现 Unlocatable 行:\n%s", out)
	}
}
