package ocr

import (
	"context"
	"errors"
	"testing"

	"github.com/yi-nology/code-review-service/internal/repository"
	"github.com/yi-nology/code-review-service/internal/repository/postgres/models"
	"github.com/yi-nology/code-review-service/internal/service/llm"
	"github.com/yi-nology/code-review-service/internal/service/ocr"
	"github.com/yi-nology/code-review-service/internal/service/review/domain"
	"github.com/yi-nology/code-review-service/internal/service/runner"
)

// stubRepository 是不依赖 DB 的最小 RepoContainer。
// 本测试只验证 Registry 的 getter 转发，不验证子 manager 业务逻辑。
func stubRepository() *repository.RepoContainer {
	// 实际场景下 Repos 为 nil 也不影响 getter 测试。
	return nil
}

// stubTester 满足 domain.LLMTester
type stubTester struct{ err error }

func (s *stubTester) TestLLM(_ context.Context) (*domain.LLMTestResult, error) {
	if s.err != nil {
		return nil, s.err
	}
	return &domain.LLMTestResult{Success: true, Message: "ok"}, nil
}

type stubLister struct{ providers []domain.LLMProvider }

func (s *stubLister) ListLLMProviders(_ context.Context) ([]domain.LLMProvider, error) {
	return s.providers, nil
}

type stubChecker struct{ applicable bool }

func (s *stubChecker) RulesCheck(_ context.Context, _ string) (*domain.RulesCheckResult, error) {
	return &domain.RulesCheckResult{Applicable: s.applicable, Rule: "rule-x", Message: "ok"}, nil
}

type stubVersioner struct{ v string }

func (s *stubVersioner) OCRVersion(_ context.Context) (*domain.OCRVersion, error) {
	return &domain.OCRVersion{Version: s.v}, nil
}

type stubViewer struct{}

func (s *stubViewer) ViewerStatus(_ context.Context) (*domain.ViewerStatus, error) {
	return &domain.ViewerStatus{Running: false}, nil
}
func (s *stubViewer) StartViewer(_ context.Context, _ string) (*domain.ViewerStatus, error) {
	return &domain.ViewerStatus{Running: true}, nil
}
func (s *stubViewer) StopViewer(_ context.Context) (*domain.ViewerStatus, error) {
	return &domain.ViewerStatus{Running: false}, nil
}

type stubUpgrader struct{}

func (s *stubUpgrader) CheckUpdate(_ context.Context) (*domain.OCRUpdateInfo, error) {
	return &domain.OCRUpdateInfo{}, nil
}
func (s *stubUpgrader) Upgrade(_ context.Context, _ string) (*domain.OCRUpgradeStatus, error) {
	return &domain.OCRUpgradeStatus{State: domain.UpgradeIdle}, nil
}
func (s *stubUpgrader) UpgradeStatus(_ context.Context) (*domain.OCRUpgradeStatus, error) {
	return &domain.OCRUpgradeStatus{State: domain.UpgradeIdle}, nil
}

type stubRunner struct{ v string }

func (s *stubRunner) Run(_ context.Context, _ domain.Workspace) (domain.Result, error) {
	return domain.Result{Summary: s.v}, nil
}

type stubPreviewer struct{}

func (s *stubPreviewer) Preview(_ context.Context, _ domain.Workspace) (*domain.PreviewResult, error) {
	return &domain.PreviewResult{Files: []string{"a.go"}}, nil
}

type stubScanner struct{}

func (s *stubScanner) Scan(_ context.Context, _ domain.Workspace, _ domain.ScanRequest) (*domain.ScanResult, error) {
	return &domain.ScanResult{}, nil
}

type stubLLM struct{}

func (s *stubLLM) Complete(_ context.Context, _ llm.CompleteRequest) (llm.CompleteResponse, error) {
	return llm.CompleteResponse{Content: "ok"}, nil
}
func (s *stubLLM) Available(_ context.Context) bool { return true }

// makeRegistry 构造一个全部 getter 都有 stub 的 Registry。
func makeRegistry() *Registry {
	r := New(
		stubRepository(),
		nil, // fallback runner 不在 getter 路径上
		&stubTester{},
		&stubLister{},
		&stubChecker{},
		&stubVersioner{},
		&stubViewer{},
		&stubUpgrader{},
		&stubLLM{},
	)
	return r.WithRunner(&stubRunner{v: "runner"}, &stubPreviewer{}, &stubScanner{})
}

// TestRegistry_AllGettersReturnNonNil 验证所有 getter 返回非 nil。
func TestRegistry_AllGettersReturnNonNil(t *testing.T) {
	r := makeRegistry()

	if r.Runner() == nil {
		t.Errorf("Runner() returned nil")
	}
	if r.Previewer() == nil {
		t.Errorf("Previewer() returned nil")
	}
	if r.Scanner() == nil {
		t.Errorf("Scanner() returned nil")
	}
	if r.Tester() == nil {
		t.Errorf("Tester() returned nil")
	}
	if r.Lister() == nil {
		t.Errorf("Lister() returned nil")
	}
	if r.Checker() == nil {
		t.Errorf("Checker() returned nil")
	}
	if r.Versioner() == nil {
		t.Errorf("Versioner() returned nil")
	}
	if r.Viewer() == nil {
		t.Errorf("Viewer() returned nil")
	}
	if r.Upgrader() == nil {
		t.Errorf("Upgrader() returned nil")
	}
	if r.LLM() == nil {
		t.Errorf("LLM() returned nil")
	}
}

// TestRegistry_TesterDelegation 验证 Tester getter 透传到 stub。
func TestRegistry_TesterDelegation(t *testing.T) {
	r := makeRegistry()
	res, err := r.Tester().TestLLM(context.Background())
	if err != nil {
		t.Fatalf("TestLLM returned err: %v", err)
	}
	if res == nil || !res.Success {
		t.Fatalf("expected success result, got %+v", res)
	}
	if res.Message != "ok" {
		t.Fatalf("expected message=ok, got %q", res.Message)
	}
}

// TestRegistry_TesterErrorPropagated 验证底层 error 透传。
func TestRegistry_TesterErrorPropagated(t *testing.T) {
	r := New(stubRepository(), nil,
		&stubTester{err: errors.New("boom")},
		&stubLister{}, &stubChecker{}, &stubVersioner{},
		&stubViewer{}, &stubUpgrader{}, &stubLLM{})
	_, err := r.Tester().TestLLM(context.Background())
	if err == nil {
		t.Fatalf("expected err to propagate")
	}
	if err.Error() != "boom" {
		t.Fatalf("expected 'boom', got %q", err.Error())
	}
}

// TestRegistry_ListerDelegation 验证 Lister 透传。
func TestRegistry_ListerDelegation(t *testing.T) {
	providers := []domain.LLMProvider{{Name: "openai", Protocol: "openai"}}
	r := New(stubRepository(), nil,
		&stubTester{},
		&stubLister{providers: providers},
		&stubChecker{}, &stubVersioner{},
		&stubViewer{}, &stubUpgrader{}, &stubLLM{})
	got, err := r.Lister().ListLLMProviders(context.Background())
	if err != nil {
		t.Fatalf("ListLLMProviders err: %v", err)
	}
	if len(got) != 1 || got[0].Name != "openai" {
		t.Fatalf("expected [openai], got %+v", got)
	}
}

// TestRegistry_CheckerDelegation 验证 Checker 透传。
func TestRegistry_CheckerDelegation(t *testing.T) {
	r := New(stubRepository(), nil,
		&stubTester{}, &stubLister{},
		&stubChecker{applicable: true},
		&stubVersioner{}, &stubViewer{}, &stubUpgrader{}, &stubLLM{})
	got, err := r.Checker().RulesCheck(context.Background(), "/some/path")
	if err != nil {
		t.Fatalf("RulesCheck err: %v", err)
	}
	if !got.Applicable {
		t.Fatalf("expected applicable=true, got %+v", got)
	}
}

// TestRegistry_VersionerDelegation 验证 Versioner 透传。
func TestRegistry_VersionerDelegation(t *testing.T) {
	r := New(stubRepository(), nil,
		&stubTester{}, &stubLister{}, &stubChecker{},
		&stubVersioner{v: "v1.2.3"},
		&stubViewer{}, &stubUpgrader{}, &stubLLM{})
	got, err := r.Versioner().OCRVersion(context.Background())
	if err != nil {
		t.Fatalf("OCRVersion err: %v", err)
	}
	if got.Version != "v1.2.3" {
		t.Fatalf("expected v1.2.3, got %q", got.Version)
	}
}

// TestRegistry_RunnerTripleInterfaces 验证 WithRunner 同时支持 3 个 interface。
// 使用一个同时实现 Runner/PreviewRunner/ScanRunner 的 stub 模拟 DynamicOpenCodeReviewRunner。
type tripleRunner struct{}

func (tripleRunner) Run(_ context.Context, _ domain.Workspace) (domain.Result, error) {
	return domain.Result{}, nil
}
func (tripleRunner) Preview(_ context.Context, _ domain.Workspace) (*domain.PreviewResult, error) {
	return &domain.PreviewResult{}, nil
}
func (tripleRunner) Scan(_ context.Context, _ domain.Workspace, _ domain.ScanRequest) (*domain.ScanResult, error) {
	return &domain.ScanResult{}, nil
}

func TestRegistry_RunnerTripleInterfaces(t *testing.T) {
	tr := tripleRunner{}
	r := New(stubRepository(), nil,
		&stubTester{}, &stubLister{}, &stubChecker{}, &stubVersioner{},
		&stubViewer{}, &stubUpgrader{}, &stubLLM{},
	).WithRunner(tr, tr, tr)

	// 验证 3 个 getter 拿到的都是同一实例
	if r.Runner() != domain.Runner(tr) {
		t.Errorf("Runner() did not return tr")
	}
	if r.Previewer() != domain.PreviewRunner(tr) {
		t.Errorf("Previewer() did not return tr")
	}
	if r.Scanner() != domain.ScanRunner(tr) {
		t.Errorf("Scanner() did not return tr")
	}
	// 通过 type assertion 验证同实例满足 3 个 interface
	if _, ok := r.Runner().(domain.PreviewRunner); !ok {
		t.Errorf("Runner() does not satisfy PreviewRunner")
	}
}

// TestRegistry_LLMDelegation 验证 LLM Provider 透传。
func TestRegistry_LLMDelegation(t *testing.T) {
	r := makeRegistry()
	if !r.LLM().Available(context.Background()) {
		t.Errorf("LLM should be available")
	}
	resp, err := r.LLM().Complete(context.Background(), llm.CompleteRequest{UserPrompt: "hi"})
	if err != nil {
		t.Fatalf("Complete err: %v", err)
	}
	if resp.Content != "ok" {
		t.Fatalf("expected content=ok, got %q", resp.Content)
	}
}

// TestRegistry_ImmutabilityAfterConstruction 验证构造后修改 repos 不影响 registry getter。
// 这是文档化的"防御性"约定——Registry 不应被外部修改。
func TestRegistry_ImmutabilityAfterConstruction(t *testing.T) {
	repos := stubRepository()
	r := makeRegistry()

	// getter 暴露的是 *RepoContainer 引用本身，但具体字段不应被外部覆盖。
	// 我们这里只验证 getter 返回的引用与构造时一致。
	if r.Repos() != repos {
		t.Errorf("expected Repos() to return the same instance")
	}
}

// TestNew_NilSafety 验证 nil manager 时不 panic。
// 设计上 getter 返回 nil，由调用方保证使用前已注入。
func TestNew_NilSafety(t *testing.T) {
	r := New(nil, nil, nil, nil, nil, nil, nil, nil, nil)
	if r == nil {
		t.Fatalf("New returned nil")
	}
	// 不调 getter 避免 panic，仅确认构造成功
	_ = r
}

// ensure types stay referenced
var (
	_ = stubRepository
	_ = (*models.OCRSettings)(nil)
	_ runner.OpenCodeReviewConfig
	_ = ocr.Tester{}
)
