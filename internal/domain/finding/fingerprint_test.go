package finding

import (
	"testing"
)

// TestFingerprint_StableAcrossReviews 验证同一处代码的同一类问题，
// 在多次评审（不同 ID、不同 body 文本、行号小幅漂移）下产生相同指纹。
// 这是「新增/存量/已修复」生命周期与误报抑制的核心前提。
func TestFingerprint_StableAcrossReviews(t *testing.T) {
	base := Finding{
		Path:     "src/auth/login.go",
		Category: "security",
		Title:    "SQL Injection",
		Line:     42,
	}
	fp1 := Fingerprint(base)

	// 不同 body（LLM 每次生成不同描述）→ 指纹应不变
	differentBody := base
	differentBody.Body = "这是一段完全不同的描述文字，解释 SQL 注入风险"
	differentBody.ID = "random-id-aaa"
	if got := Fingerprint(differentBody); got != fp1 {
		t.Errorf("fingerprint changed with different body/ID:\n  base=%s\n  diff=%s", fp1, got)
	}

	// 行号小幅漂移（同桶 40-49）→ 指纹应不变
	lineDrift := base
	lineDrift.Line = 45 // 仍在 bucket 4 (40-49)
	if got := Fingerprint(lineDrift); got != fp1 {
		t.Errorf("fingerprint changed with line drift within bucket:\n  base=%s\n  drift=%s", fp1, got)
	}

	// severity 变化 → 指纹应不变（同一问题，严重级别判定不应影响"是不是同一个"）
	differentSeverity := base
	differentSeverity.Severity = "critical" // 原来是空
	if got := Fingerprint(differentSeverity); got != fp1 {
		t.Errorf("fingerprint changed with severity:\n  base=%s\n  sev=%s", fp1, got)
	}

	// title 大小写/空白差异 → 指纹应不变（规范化签名）
	titleVariant := base
	titleVariant.Title = "  sql   injection  "
	if got := Fingerprint(titleVariant); got != fp1 {
		t.Errorf("fingerprint changed with title whitespace/case:\n  base=%s\n  variant=%s", fp1, got)
	}

	// 路径格式差异（./ 前缀、反斜杠）→ 指纹应不变
	pathVariant := base
	pathVariant.Path = "./src/auth/login.go"
	if got := Fingerprint(pathVariant); got != fp1 {
		t.Errorf("fingerprint changed with path format:\n  base=%s\n  variant=%s", fp1, got)
	}
}

// TestFingerprint_DifferentProblemsDifferentFingerprint 验证不同问题产生不同指纹。
func TestFingerprint_DifferentProblemsDifferentFingerprint(t *testing.T) {
	base := Finding{Path: "src/auth/login.go", Category: "security", Title: "SQL Injection", Line: 42}

	cases := []struct {
		name   string
		modify func(Finding) Finding
	}{
		{"different path", func(f Finding) Finding { f.Path = "src/auth/logout.go"; return f }},
		{"different category", func(f Finding) Finding { f.Category = "performance"; return f }},
		{"different title", func(f Finding) Finding { f.Title = "XSS"; return f }},
		{"different line bucket", func(f Finding) Finding { f.Line = 82; return f }}, // bucket 8 vs 4
		{"different line bucket 2", func(f Finding) Finding { f.Line = 55; return f }},
	}
	baseFP := Fingerprint(base)
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Fingerprint(tc.modify(base))
			if got == baseFP {
				t.Errorf("expected different fingerprint for %s, got same as base (%s)", tc.name, baseFP)
			}
		})
	}
}

// TestFingerprint_LineBucketBoundary 验证行桶边界：相邻桶不同，同桶相同。
func TestFingerprint_LineBucketBoundary(t *testing.T) {
	base := Finding{Path: "a.go", Category: "bug", Title: "nil pointer", Line: 39} // bucket 3
	bucketEnd := base
	bucketEnd.Line = 39 // 同桶
	nextBucket := base
	nextBucket.Line = 40 // 下一桶 (bucket 4)

	fpBase := Fingerprint(base)
	if Fingerprint(bucketEnd) != fpBase {
		t.Error("line 39 should be same bucket as base 39")
	}
	if Fingerprint(nextBucket) == fpBase {
		t.Error("line 40 should be different bucket from 39")
	}
}

// TestFingerprint_LineBucketSize 验证桶大小为 10。
func TestFingerprint_LineBucketSize(t *testing.T) {
	if got := lineBucket(0); got != "*" {
		t.Errorf("lineBucket(0) = %q, want *", got)
	}
	if got := lineBucket(-5); got != "*" {
		t.Errorf("lineBucket(-5) = %q, want *", got)
	}
	if got := lineBucket(7); got != "0" {
		t.Errorf("lineBucket(7) = %q, want 0", got)
	}
	if got := lineBucket(25); got != "2" {
		t.Errorf("lineBucket(25) = %q, want 2", got)
	}
}

// TestFingerprint_Format 验证指纹是 16 字符 hex 串。
func TestFingerprint_Format(t *testing.T) {
	fp := Fingerprint(Finding{Path: "a.go", Category: "bug", Title: "x", Line: 1})
	if len(fp) != 16 {
		t.Errorf("fingerprint length = %d, want 16", len(fp))
	}
	for _, c := range fp {
		isHex := (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')
		if !isHex {
			t.Errorf("fingerprint contains non-hex char %q in %q", c, fp)
		}
	}
}

// TestComputeFingerprints 验证批量计算填充所有 findings。
func TestComputeFingerprints(t *testing.T) {
	findings := []Finding{
		{Path: "a.go", Category: "bug", Title: "x", Line: 1},
		{Path: "b.go", Category: "sec", Title: "y", Line: 2},
		{},
	}
	ComputeFingerprints(findings)
	for i, f := range findings {
		if f.Fingerprint == "" {
			t.Errorf("finding[%d] fingerprint not set", i)
		}
	}
	// 两个不同问题应有不同指纹
	if findings[0].Fingerprint == findings[1].Fingerprint {
		t.Error("different findings should have different fingerprints")
	}
	// 空 finding 也应有指纹（基于 path=""+category=""+title=""）
	if findings[2].Fingerprint == "" {
		t.Error("empty finding should still get a fingerprint")
	}
}
