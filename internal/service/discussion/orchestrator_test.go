package discussion

import (
	"context"
	"testing"

	"github.com/yi-nology/code-review-service/internal/service/llm"
)

// TestRunDiscussionDegradedWithNoopProvider 验证 LLM 不可用时自动降级到模板逻辑：
// 不报错、不产出新发现（NewFindings 为空），但仍产出 consensus/disagreement 聚合。
func TestRunDiscussionDegradedWithNoopProvider(t *testing.T) {
	o := NewOrchestrator(DefaultDiscussionConfig(), llm.NewNoopProvider())

	findings := []Finding{
		{ID: "f1", Severity: "critical", Path: "a.go", Line: 1, EndLine: 1, Body: "严重 bug", Agents: []string{"a1"}},
		{ID: "f2", Severity: "major", Path: "b.go", Line: 2, EndLine: 2, Body: "性能问题", Agents: []string{"a2"}},
	}

	result, err := o.RunDiscussion(context.Background(), findings, []string{"a1", "a2"}, nil)
	if err != nil {
		t.Fatalf("degraded RunDiscussion should not error, got %v", err)
	}
	if result == nil {
		t.Fatal("result must not be nil")
	}
	// 降级路径不产出 SURFACE 新发现
	if len(result.NewFindings) != 0 {
		t.Fatalf("degraded mode must not produce new findings, got %d", len(result.NewFindings))
	}
	// 降级路径应产出汇总
	if result.Summary == "" {
		t.Fatal("degraded mode should still produce a summary")
	}
}

// TestRunDiscussionSkipsWhenDisabled 验证 discussion 被禁用时立即返回。
func TestRunDiscussionSkipsWhenDisabled(t *testing.T) {
	cfg := DefaultDiscussionConfig()
	cfg.Enabled = false
	o := NewOrchestrator(cfg, llm.NewNoopProvider())

	result, err := o.RunDiscussion(context.Background(), nil, []string{"a1"}, nil)
	if err != nil {
		t.Fatalf("disabled discussion should not error, got %v", err)
	}
	if result == nil || result.Summary != "discussion disabled" {
		t.Fatalf("expected disabled summary, got %+v", result)
	}
}

// TestRunDiscussionTooFewAgents 验证 agent 数不足时跳过。
func TestRunDiscussionTooFewAgents(t *testing.T) {
	o := NewOrchestrator(DefaultDiscussionConfig(), llm.NewNoopProvider())

	result, err := o.RunDiscussion(context.Background(), nil, []string{"only-one"}, nil)
	if err != nil {
		t.Fatalf("should not error, got %v", err)
	}
	if result == nil || result.Summary == "" {
		t.Fatal("expected not-enough-agents summary")
	}
}
