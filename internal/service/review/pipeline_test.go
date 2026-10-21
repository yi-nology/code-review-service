package review

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/yi-nology/code-review-service/internal/domain/ocrasset"
	"github.com/yi-nology/code-review-service/internal/domain/ocrsettings"
	"github.com/yi-nology/code-review-service/internal/domain/projectpolicy"
	"github.com/yi-nology/code-review-service/internal/domain/review"
	"github.com/yi-nology/code-review-service/internal/domain/reviewer"
	"github.com/yi-nology/code-review-service/internal/domain/rules"
	"github.com/yi-nology/code-review-service/internal/repository/repoerr"
)

// ---- 测试替身 ----

// pipeStore 是 ReviewStateStore 的内存实现，记录审计事件。
type pipeStore struct {
	record   *domain.Record
	events   []domain.Event
	ocrInput string
	saveErr  error
}

func newFakeStore() *pipeStore {
	return &pipeStore{record: &domain.Record{Response: domain.Response{}}}
}

func (f *pipeStore) GetByReviewID(context.Context, string) (*domain.Record, error) {
	return f.record, nil
}

func (f *pipeStore) Create(context.Context, *domain.Record) error { return nil }

func (f *pipeStore) Save(_ context.Context, r *domain.Record) error {
	if f.saveErr != nil {
		return f.saveErr
	}
	f.record = r
	return nil
}

func (f *pipeStore) UpdateInTransaction(_ context.Context, _ string, fn func(*domain.Record) error) error {
	before := len(f.record.Response.History)
	if err := fn(f.record); err != nil {
		return err
	}
	// 只收本次事务新增的事件（record 里的 history 是累积的）。
	f.events = append(f.events, f.record.Response.History[before:]...)
	return nil
}

// stages 返回已记录的审计环节序列。
func (f *pipeStore) stages() []string {
	out := make([]string, 0, len(f.events))
	for _, e := range f.events {
		out = append(out, e.Stage)
	}
	return out
}

// pipeProvider 是 domain.Provider 的替身。
type pipeProvider struct {
	workspace domain.Workspace
	err       error
}

func (f *pipeProvider) Prepare(context.Context, domain.Request) (domain.Workspace, error) {
	if f.err != nil {
		return domain.Workspace{}, f.err
	}
	return f.workspace, nil
}

// pipePolicyRepo 是 ProjectPolicyRepository 的替身。
type pipePolicyRepo struct {
	policy projectpolicy.ProjectPolicy
	err    error
}

func (f *pipePolicyRepo) List(context.Context) ([]projectpolicy.ProjectPolicy, error) {
	return nil, nil
}
func (f *pipePolicyRepo) GetByProviderRepo(context.Context, string, string) (*projectpolicy.ProjectPolicy, error) {
	return nil, repoerr.ErrNotFound
}
func (f *pipePolicyRepo) GetOrCreateDefault(context.Context, string, string) (projectpolicy.ProjectPolicy, error) {
	if f.err != nil {
		return projectpolicy.ProjectPolicy{}, f.err
	}
	return f.policy, nil
}
func (f *pipePolicyRepo) Save(context.Context, *projectpolicy.ProjectPolicy) error { return nil }
func (f *pipePolicyRepo) Delete(context.Context, string, string) error             { return nil }

// pipeGlobalRulesRepo 是 GlobalRulesRepository 的替身。
type pipeGlobalRulesRepo struct {
	global *rules.Global
}

func (f *pipeGlobalRulesRepo) Get(context.Context) (*rules.Global, error) { return f.global, nil }
func (f *pipeGlobalRulesRepo) Save(context.Context, *rules.Global) error  { return nil }

// pipeRulePackageRepo 是 OCRRulePackageRepository 的替身。
type pipeRulePackageRepo struct{}

func (f *pipeRulePackageRepo) List(context.Context) ([]ocrasset.RulePackage, error) {
	return nil, nil
}
func (f *pipeRulePackageRepo) GetByID(context.Context, uint) (*ocrasset.RulePackage, error) {
	return nil, repoerr.ErrNotFound
}
func (f *pipeRulePackageRepo) Save(context.Context, *ocrasset.RulePackage) error { return nil }
func (f *pipeRulePackageRepo) InsertIfAbsent(context.Context, *ocrasset.RulePackage) (uint, error) {
	return 0, nil
}
func (f *pipeRulePackageRepo) ListVersions(context.Context, uint) ([]ocrasset.RulePackageVersion, error) {
	return nil, nil
}
func (f *pipeRulePackageRepo) RestoreVersion(context.Context, uint, int) (*ocrasset.RulePackage, error) {
	return nil, repoerr.ErrNotFound
}
func (f *pipeRulePackageRepo) Delete(context.Context, uint) error { return nil }

// pipeAgentRepo 是 ReviewerAgentRepository 的替身。
type pipeAgentRepo struct{}

func (f *pipeAgentRepo) List(context.Context) ([]reviewer.Agent, error) { return nil, nil }
func (f *pipeAgentRepo) GetByID(context.Context, uint) (*reviewer.Agent, error) {
	return nil, repoerr.ErrNotFound
}
func (f *pipeAgentRepo) GetByName(context.Context, string) (*reviewer.Agent, error) {
	return nil, repoerr.ErrNotFound
}
func (f *pipeAgentRepo) Create(context.Context, *reviewer.Agent) error { return nil }
func (f *pipeAgentRepo) Update(context.Context, *reviewer.Agent) error { return nil }
func (f *pipeAgentRepo) Delete(context.Context, uint) error            { return nil }
func (f *pipeAgentRepo) ListEnabled(context.Context) ([]reviewer.Agent, error) {
	return nil, nil
}

// pipeOCRSettingsRepo 是 OCRSettingsRepository 的替身。
type pipeOCRSettingsRepo struct {
	settings *ocrsettings.Settings
}

func (f *pipeOCRSettingsRepo) Get(context.Context) (*ocrsettings.Settings, error) {
	return f.settings, nil
}
func (f *pipeOCRSettingsRepo) Save(context.Context, *ocrsettings.Settings) error { return nil }

// pipeAgentRegistry 是 AgentRegistry 的替身。
type pipeAgentRegistry struct{ names []string }

func (f *pipeAgentRegistry) AgentNames() []string { return f.names }

// pipeMemory 是 MemoryRecaller 的替身。
type pipeMemory struct {
	text string
	err  error
	got  string
}

func (f *pipeMemory) RecallReviewContext(_ context.Context, _, _, _, query string) (string, error) {
	f.got = query
	return f.text, f.err
}

// newTestPipeline 构造一个可运行的管线（全部依赖为替身）。
func newTestPipeline(store ReviewStateStore, provider domain.Provider) *Pipeline {
	return NewPipeline(
		store,
		&pipePolicyRepo{},
		&pipeGlobalRulesRepo{},
		&pipeRulePackageRepo{},
		&pipeAgentRepo{},
		&pipeOCRSettingsRepo{},
		&pipeAgentRegistry{names: []string{"a1", "a2"}},
		nil,
	)
}

// ---- 用例 ----

// TestPipeline_Step1Failure 回归：克隆失败必须带「repository」环节标签。
//
// 环节标签要跨过管线边界回到引擎侧（失败上报与审计按环节归类），
// 丢了它失败就只能笼统记成「pipeline」。
func TestPipeline_Step1Failure(t *testing.T) {
	store := newFakeStore()
	p := newTestPipeline(store, &pipeProvider{err: errors.New("clone failed")})

	_, err := p.Run(context.Background(), PipelineInput{
		ReviewID: "r1",
		Request:  domain.Request{Provider: "github", Repo: "o/r"},
		Provider: &pipeProvider{err: errors.New("clone failed")},
	})
	if err == nil {
		t.Fatal("克隆失败应返回错误")
	}
	if got := StageOf(err); got != "repository" {
		t.Errorf("失败环节 = %q, want %q", got, "repository")
	}
}

// TestPipeline_ScopeFailure 回归：scope 过滤失败带「scope」环节标签。
func TestPipeline_ScopeFailure(t *testing.T) {
	dir := t.TempDir() // 不是 git 仓库 → ApplyProjectScope 会失败
	store := newFakeStore()
	policyRepo := &pipePolicyRepo{policy: projectpolicy.ProjectPolicy{
		Provider:    "github",
		Repo:        "o/r",
		IgnorePaths: json.RawMessage(`["docs/**"]`),
	}}
	p := NewPipeline(store, policyRepo, &pipeGlobalRulesRepo{}, &pipeRulePackageRepo{},
		&pipeAgentRepo{}, &pipeOCRSettingsRepo{}, nil, nil)

	_, err := p.Run(context.Background(), PipelineInput{
		ReviewID: "r1",
		Request:  domain.Request{Provider: "github", Repo: "o/r"},
		Provider: &pipeProvider{workspace: domain.Workspace{Dir: dir}},
	})
	if err == nil {
		t.Fatal("scope 过滤失败应返回错误")
	}
	if got := StageOf(err); got != "scope" {
		t.Errorf("失败环节 = %q, want %q", got, "scope")
	}
}

// TestPipeline_RecordsAuditSteps 验证审计环节序列（可观测性不能因为搬家丢步骤）。
func TestPipeline_RecordsAuditSteps(t *testing.T) {
	store := newFakeStore()
	dir := t.TempDir()
	p := newTestPipeline(store, &pipeProvider{workspace: domain.Workspace{Dir: dir}})

	if _, err := p.Run(context.Background(), PipelineInput{
		ReviewID:         "r1",
		Request:          domain.Request{Provider: "github", Repo: "o/r"},
		Provider:         &pipeProvider{workspace: domain.Workspace{Dir: dir}},
		ProgressReporter: nil,
	}); err != nil {
		t.Fatalf("管线应成功: %v", err)
	}

	stages := store.stages()
	want := []string{"repository", "policy", "rules", "agent", "ocr_config"}
	if len(stages) != len(want) {
		t.Fatalf("审计环节 = %v, want %v", stages, want)
	}
	for i := range want {
		if stages[i] != want[i] {
			t.Errorf("第 %d 个环节 = %q, want %q", i, stages[i], want[i])
		}
	}
}

// TestPipeline_MergesMemoryContext 验证召回的记忆并进背景**末尾**。
//
// 背景是模型读的「本次评审须知」，记忆是历史经验——排在后面才���盖过
// 项目规则与需求。
func TestPipeline_MergesMemoryContext(t *testing.T) {
	store := newFakeStore()
	dir := t.TempDir()
	mem := &pipeMemory{text: "历史记忆：该项目用 Go，错误处理用 oekit/pkg/errors"}
	p := NewPipeline(store, &pipePolicyRepo{}, &pipeGlobalRulesRepo{}, &pipeRulePackageRepo{},
		&pipeAgentRepo{}, &pipeOCRSettingsRepo{}, &pipeAgentRegistry{}, mem)

	out, err := p.Run(context.Background(), PipelineInput{
		ReviewID: "r1",
		Request:  domain.Request{Provider: "github", Repo: "o/r"},
		Provider: &pipeProvider{workspace: domain.Workspace{Dir: dir}},
	})
	if err != nil {
		t.Fatalf("管线应成功: %v", err)
	}
	if out.MemoryContext != mem.text {
		t.Errorf("记忆上下文 = %q, want %q", out.MemoryContext, mem.text)
	}
	bg := out.Workspace.OCR.Background
	if bg == "" {
		t.Fatal("背景应包含记忆")
	}
	if len(bg) < len(mem.text) || bg[len(bg)-len(mem.text):] != mem.text {
		t.Errorf("记忆应在背景末尾, got %q", bg)
	}
}

// TestPipeline_MemoryQueryUsesRepoAndChangedPaths 验证召回查询词由「仓库 + 变更文件」构成。
func TestPipeline_MemoryQueryUsesRepoAndChangedPaths(t *testing.T) {
	store := newFakeStore()
	dir := t.TempDir()
	mem := &pipeMemory{}
	p := NewPipeline(store, &pipePolicyRepo{}, &pipeGlobalRulesRepo{}, &pipeRulePackageRepo{},
		&pipeAgentRepo{}, &pipeOCRSettingsRepo{}, &pipeAgentRegistry{}, mem)

	diff := "diff --git a/a.go b/a.go\n+++ b/a.go\n@@ -1 +1 @@\n-x\n+y\n"
	_, err := p.Run(context.Background(), PipelineInput{
		ReviewID: "r1",
		Request:  domain.Request{Provider: "github", Repo: "o/r"},
		Provider: &pipeProvider{workspace: domain.Workspace{Dir: dir, Diff: diff}},
	})
	if err != nil {
		t.Fatalf("管线应成功: %v", err)
	}
	if mem.got != "o/r a.go" {
		t.Errorf("召回查询词 = %q, want %q", mem.got, "o/r a.go")
	}
}

// TestPipeline_CleanupRemovesWorkspace 验证清理责任在调用方：管线返回的 Cleanup
// 要真的删掉工作区（工作区在引擎执行期间必须存活，所以清理不能留在管线内部）。
func TestPipeline_CleanupRemovesWorkspace(t *testing.T) {
	store := newFakeStore()
	dir := t.TempDir()
	work := filepath.Join(dir, "ws")
	if err := os.MkdirAll(work, 0o755); err != nil {
		t.Fatal(err)
	}
	p := newTestPipeline(store, &pipeProvider{workspace: domain.Workspace{Dir: work}})

	out, err := p.Run(context.Background(), PipelineInput{
		ReviewID: "r1",
		Request:  domain.Request{Provider: "github", Repo: "o/r"},
		Provider: &pipeProvider{workspace: domain.Workspace{Dir: work}},
	})
	if err != nil {
		t.Fatalf("管线应成功: %v", err)
	}
	if _, err := os.Stat(work); err != nil {
		t.Fatalf("工作区应仍存在（引擎执行期间必须存活）: %v", err)
	}
	if out.Cleanup == nil {
		t.Fatal("Cleanup 不应为 nil")
	}
	out.Cleanup()
	if _, err := os.Stat(work); !os.IsNotExist(err) {
		t.Errorf("Cleanup 后工作区应被删除, stat err=%v", err)
	}
}

// TestPipeline_WritesOCRInput 验证 OCR 执行入参快照落库（resume 与审计共用）。
func TestPipeline_WritesOCRInput(t *testing.T) {
	store := newFakeStore()
	dir := t.TempDir()
	p := newTestPipeline(store, &pipeProvider{workspace: domain.Workspace{Dir: dir}})

	if _, err := p.Run(context.Background(), PipelineInput{
		ReviewID: "r1",
		Request:  domain.Request{Provider: "github", Repo: "o/r", BaseSHA: "aaa", HeadSHA: "bbb"},
		Provider: &pipeProvider{workspace: domain.Workspace{Dir: dir, BaseSHA: "aaa", HeadSHA: "bbb"}},
	}); err != nil {
		t.Fatalf("管线应成功: %v", err)
	}
	if store.record.RawOCRInput == nil {
		t.Fatal("OCR 输入快照未落库")
	}
	var in domain.OCRInput
	if err := json.Unmarshal(store.record.RawOCRInput, &in); err != nil {
		t.Fatalf("OCR 输入快照不是合法 JSON: %v", err)
	}
	if in.Workspace.BaseSHA != "aaa" || in.Workspace.HeadSHA != "bbb" {
		t.Errorf("OCR 输入快照的 SHA = %q/%q, want aaa/bbb", in.Workspace.BaseSHA, in.Workspace.HeadSHA)
	}
}
