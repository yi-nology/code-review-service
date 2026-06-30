package ocr

import (
	"context"
	"testing"
	"time"

	"github.com/yi-nology/code-review-service/internal/service/llm"
	"github.com/yi-nology/code-review-service/internal/service/review/domain"
)

// TestRegistry_LLMAvailableAndComplete 验证 LLM 能力的完整调用链。
// 该测试不需要真实 DB：所有 manager 用 stub。
func TestRegistry_LLMAvailableAndComplete(t *testing.T) {
	r := New(nil, nil,
		&stubTester{},
		&stubLister{},
		&stubChecker{applicable: true},
		&stubVersioner{v: "v0.1.0"},
		&stubViewer{},
		&stubUpgrader{},
		&stubLLM{},
	)
	_ = r.WithRunner(&stubRunner{v: "ok"}, &stubPreviewer{}, &stubScanner{})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// LLM 调用
	resp, err := r.LLM().Complete(ctx, llm.CompleteRequest{
		SystemPrompt: "you are a code reviewer",
		UserPrompt:   "review this PR",
	})
	if err != nil {
		t.Fatalf("LLM.Complete err: %v", err)
	}
	if resp.Content != "ok" {
		t.Fatalf("expected content=ok, got %q", resp.Content)
	}
	if !r.LLM().Available(ctx) {
		t.Errorf("LLM should be available")
	}
}

// TestRegistry_AllDomainInterfaces 验证每个 getter 满足对应的 domain interface。
// 该测试在编译期保证接口契约正确（interface assertion 在编译失败时报错）。
func TestRegistry_AllDomainInterfaces(t *testing.T) {
	r := New(nil, nil,
		&stubTester{}, &stubLister{}, &stubChecker{}, &stubVersioner{},
		&stubViewer{}, &stubUpgrader{}, &stubLLM{},
	)
	_ = r.WithRunner(&stubRunner{}, &stubPreviewer{}, &stubScanner{})

	// 编译期断言：每个 getter 返回的值满足 domain interface
	var (
		_ domain.Runner             = r.Runner()
		_ domain.PreviewRunner      = r.Previewer()
		_ domain.ScanRunner         = r.Scanner()
		_ domain.LLMTester          = r.Tester()
		_ domain.LLMProviderLister  = r.Lister()
		_ domain.RulesChecker       = r.Checker()
		_ domain.OCRVersioner       = r.Versioner()
		_ domain.ViewerManager      = r.Viewer()
		_ domain.OCRUpgrader        = r.Upgrader()
		_ llm.Provider              = r.LLM()
	)
	_ = r
}

// TestRegistry_GettersAreImmutable 验证构造后 getter 返回的引用是稳定的。
// 这保证业务侧持有 getter 引用后，registry 后续操作不会替换底层实现。
func TestRegistry_GettersAreImmutable(t *testing.T) {
	r := New(nil, nil,
		&stubTester{}, &stubLister{}, &stubChecker{}, &stubVersioner{},
		&stubViewer{}, &stubUpgrader{}, &stubLLM{},
	)
	_ = r.WithRunner(&stubRunner{}, &stubPreviewer{}, &stubScanner{})

	t1 := r.Tester()
	t2 := r.Tester()
	if t1 != t2 {
		t.Errorf("Tester() returned different references on consecutive calls")
	}

	v1 := r.Versioner()
	v2 := r.Versioner()
	if v1 != v2 {
		t.Errorf("Versioner() returned different references on consecutive calls")
	}
}
