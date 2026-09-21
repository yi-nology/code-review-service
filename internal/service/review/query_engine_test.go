package review_test

import (
	"context"
	"testing"

	"github.com/yi-nology/code-review-service/internal/service/review"
)

// engine 过滤的 service 层校验：非法值 fail-fast 报 400 语义错误，
// 合法值（含空 = 不过滤）放行到仓储层（过滤语义由 postgres 集成测试覆盖）。
func TestListEngineFilterValidation(t *testing.T) {
	repo := &mockReviewRepo{}
	svc := newTestService(repo, blockingRunner{})

	for _, engine := range []string{"gpt", "claude", "ocr; drop"} {
		if _, err := svc.List(context.Background(), review.ListOptions{Limit: 50, Engine: engine}); err == nil {
			t.Errorf("List(Engine=%q) expected error, got nil", engine)
		}
	}
	for _, engine := range []string{"", "ocr", "argus", "Argus"} {
		if _, err := svc.List(context.Background(), review.ListOptions{Limit: 50, Engine: engine}); err != nil {
			t.Errorf("List(Engine=%q) unexpected error: %v", engine, err)
		}
	}
}
