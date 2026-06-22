package writeback

import (
	"context"
	"strings"
	"testing"

	"github.com/yi-nology/code-review-service/internal/service/llm"
	"github.com/yi-nology/code-review-service/internal/service/review"
)

// TestTranslatorDegradedToTemplate 验证 LLM 不可用时降级为模板格式（不报错、产出可读 Markdown）。
func TestTranslatorDegradedToTemplate(t *testing.T) {
	tr := NewTranslator(llm.NewNoopProvider())

	findings := []review.Finding{
		{Severity: review.SeverityCritical, Path: "a.go", Line: 5, Title: "空指针"},
		{Severity: review.SeverityMajor, Path: "b.go", Line: 10, Title: "性能问题"},
	}

	out, err := tr.TranslateToHumanVoice(context.Background(), findings, "汇总")
	if err != nil {
		t.Fatalf("degraded translation should not error, got %v", err)
	}
	if strings.TrimSpace(out.Markdown) == "" {
		t.Fatal("degraded mode must produce non-empty markdown")
	}
	if !strings.Contains(out.Markdown, "a.go") {
		t.Fatalf("degraded markdown should contain finding path, got: %s", out.Markdown)
	}
}

// TestTranslatorEmptyFindings 验证无发现时仍不报错。
func TestTranslatorEmptyFindings(t *testing.T) {
	tr := NewTranslator(llm.NewNoopProvider())
	out, err := tr.TranslateToHumanVoice(context.Background(), nil, "")
	if err != nil {
		t.Fatalf("should not error, got %v", err)
	}
	if out.Markdown == "" {
		t.Fatal("expected fallback markdown even for empty findings")
	}
}

// TestExtractVerdict 验证从 Markdown 提取结论标记。
func TestExtractVerdict(t *testing.T) {
	cases := map[string]string{
		"APPROVE this PR":        "APPROVE",
		"REQUEST_CHANGES needed": "REQUEST_CHANGES",
		"NEEDS_DISCUSSION":       "NEEDS_DISCUSSION",
		"looks good generally":   "",
	}
	for input, want := range cases {
		if got := extractVerdict(input); got != want {
			t.Errorf("extractVerdict(%q) = %q, want %q", input, got, want)
		}
	}
}
