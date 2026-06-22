package requirements

import (
	"context"
	"testing"

	"github.com/yi-nology/code-review-service/internal/service/llm"
)

// TestGenerateClarifyingQuestionsDegraded 验证 LLM 不可用时返回 nil（降级，不阻塞主流程）。
func TestGenerateClarifyingQuestionsDegraded(t *testing.T) {
	qs := GenerateClarifyingQuestions(
		context.Background(),
		llm.NewNoopProvider(),
		[]string{"每秒最多 100 请求"},
		[]Finding{{ID: "1", Severity: "major", Path: "a.go", Title: "x"}},
	)
	if qs != nil {
		t.Fatalf("expected nil when LLM unavailable, got %v", qs)
	}
}

// TestGenerateClarifyingQuestionsSkipsWhenNoReqs 验证无需求时不调用 LLM。
func TestGenerateClarifyingQuestionsSkipsWhenNoReqs(t *testing.T) {
	qs := GenerateClarifyingQuestions(
		context.Background(),
		llm.NewNoopProvider(),
		nil,
		nil,
	)
	if qs != nil {
		t.Fatalf("expected nil when no requirements, got %v", qs)
	}
}
