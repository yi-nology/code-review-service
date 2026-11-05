package review_test

import (
	"context"
	"sync"
	"time"

	"github.com/yi-nology/code-review-service/internal/domain/page"

	"github.com/yi-nology/code-review-service/internal/domain/ocrsettings"
	"github.com/yi-nology/code-review-service/internal/domain/projectpolicy"
	"github.com/yi-nology/code-review-service/internal/domain/review"
	"github.com/yi-nology/code-review-service/internal/repository/repoerr"
	"github.com/yi-nology/code-review-service/internal/service/policy"
	"github.com/yi-nology/code-review-service/internal/service/progress"
	"github.com/yi-nology/code-review-service/internal/service/review"

	"github.com/jackc/pgx/v5/pgconn"
)

type mockReviewRepo struct {
	mu          sync.Mutex
	reviews     []*domain.Record
	saveErr     error
	createErr   error
	claimFn     func(ctx context.Context, limit int) ([]domain.Record, error)
	failStaleFn func(ctx context.Context, before time.Time) ([]domain.Record, error)
	listFn      func(ctx context.Context, opts domain.ListOptions) (*page.Page[domain.Record], error)
	touchCalls  int
}

func (m *mockReviewRepo) Create(_ context.Context, r *domain.Record) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.createErr != nil {
		return m.createErr
	}
	if r.IdempotencyKey != "" {
		for _, existing := range m.reviews {
			if existing.IdempotencyKey != "" && existing.IdempotencyKey == r.IdempotencyKey {
				return &pgconn.PgError{Code: "23505"}
			}
		}
	}
	m.reviews = append(m.reviews, r)
	return nil
}

func (m *mockReviewRepo) Save(_ context.Context, r *domain.Record) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.saveErr != nil {
		return m.saveErr
	}
	for i, existing := range m.reviews {
		if existing.ID == r.ID {
			m.reviews[i] = r
			return nil
		}
	}
	m.reviews = append(m.reviews, r)
	return nil
}

func (m *mockReviewRepo) GetByReviewID(_ context.Context, reviewID string) (*domain.Record, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, r := range m.reviews {
		if r.ID == reviewID {
			return r, nil
		}
	}
	return nil, repoerr.ErrNotFound
}

func (m *mockReviewRepo) GetByIdempotencyKey(_ context.Context, key string) (*domain.Record, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, r := range m.reviews {
		if r.IdempotencyKey != "" && r.IdempotencyKey == key {
			return r, nil
		}
	}
	return nil, repoerr.ErrNotFound
}

func (m *mockReviewRepo) GetActiveByProviderRepo(_ context.Context, provider, repo, pullRequestID, mergeRequestIID string) (*domain.Record, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, r := range m.reviews {
		if r.Provider != provider || r.Repo != repo {
			continue
		}
		if r.Status != "queued" && r.Status != "running" {
			continue
		}
		if pullRequestID != "" && r.PullRequestID != pullRequestID {
			continue
		}
		if mergeRequestIID != "" && r.MergeRequestIID != mergeRequestIID {
			continue
		}
		return r, nil
	}
	return nil, repoerr.ErrNotFound
}

func (m *mockReviewRepo) List(_ context.Context, opts domain.ListOptions) (*page.Page[domain.Record], error) {
	m.mu.Lock()
	fn := m.listFn
	m.mu.Unlock()
	if fn != nil {
		return fn(context.Background(), opts)
	}
	return &page.Page[domain.Record]{Items: nil, Total: 0}, nil
}

func (m *mockReviewRepo) AggregateStats(_ context.Context, _ domain.ListOptions) (*domain.AggregateStats, error) {
	return &domain.AggregateStats{}, nil
}

func (m *mockReviewRepo) ProjectOCRMetrics(_ context.Context, _ domain.ProjectStatsOptions) (*domain.ProjectOCRMetrics, error) {
	return &domain.ProjectOCRMetrics{}, nil
}

func (m *mockReviewRepo) ProjectOCRTrend(_ context.Context, _ domain.ProjectStatsOptions) ([]domain.ProjectOCRTrendPoint, error) {
	return nil, nil
}

func (m *mockReviewRepo) ClaimQueued(ctx context.Context, limit int) ([]domain.Record, error) {
	m.mu.Lock()
	fn := m.claimFn
	m.mu.Unlock()
	if fn != nil {
		return fn(ctx, limit)
	}
	return nil, nil
}

func (m *mockReviewRepo) RequeueStale(_ context.Context, _ time.Time) (int64, error) {
	return 0, nil
}

func (m *mockReviewRepo) FailStaleRunning(ctx context.Context, before time.Time) ([]domain.Record, error) {
	m.mu.Lock()
	fn := m.failStaleFn
	m.mu.Unlock()
	if fn != nil {
		return fn(ctx, before)
	}
	return nil, nil
}

func (m *mockReviewRepo) TouchRunning(_ context.Context, _ string) error {
	m.mu.Lock()
	m.touchCalls++
	m.mu.Unlock()
	return nil
}

func (m *mockReviewRepo) QueueDepth(_ context.Context) (int64, error) {
	return 0, nil
}

func (m *mockReviewRepo) RequeueFromDeadLetter(_ context.Context, _ string) error {
	return nil
}

func (m *mockReviewRepo) UpdateInTransaction(_ context.Context, reviewID string, fn func(review *domain.Record) error) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, r := range m.reviews {
		if r.ID == reviewID {
			return fn(r)
		}
	}
	return repoerr.ErrNotFound
}

func (m *mockReviewRepo) snapshot() []*domain.Record {
	m.mu.Lock()
	defer m.mu.Unlock()
	// 返回值的深拷贝而非原始指针：async scan goroutine 会在锁外通过
	// UpdateInTransaction 就地改写这些对象，若 snapshot 只复制指针切片，
	// 测试 goroutine 读取字段时与异步写并发会造成数据竞争（race）。
	out := make([]*domain.Record, len(m.reviews))
	for i, r := range m.reviews {
		clone := *r
		out[i] = &clone
	}
	return out
}

type mockOCRSettingsRepo struct {
	settings *ocrsettings.Settings
	err      error
}

func (m *mockOCRSettingsRepo) Get(_ context.Context) (*ocrsettings.Settings, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.settings, nil
}

func (m *mockOCRSettingsRepo) Save(_ context.Context, _ *ocrsettings.Settings) error {
	return nil
}

type mockProjectPolicyRepo struct {
	policy *projectpolicy.ProjectPolicy
	err    error
}

func (m *mockProjectPolicyRepo) List(_ context.Context) ([]projectpolicy.ProjectPolicy, error) {
	return nil, nil
}

func (m *mockProjectPolicyRepo) GetByProviderRepo(_ context.Context, _, _ string) (*projectpolicy.ProjectPolicy, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.policy, nil
}

func (m *mockProjectPolicyRepo) GetOrCreateDefault(_ context.Context, provider, repo string) (projectpolicy.ProjectPolicy, error) {
	if m.err != nil {
		return projectpolicy.NewDefault(provider, repo), m.err
	}
	if m.policy != nil {
		return *m.policy, nil
	}
	return projectpolicy.NewDefault(provider, repo), nil
}

func (m *mockProjectPolicyRepo) Save(_ context.Context, _ *projectpolicy.ProjectPolicy) error {
	return nil
}

func (m *mockProjectPolicyRepo) Delete(_ context.Context, _, _ string) error {
	return nil
}

type testProviders struct {
	providers map[string]domain.Provider
}

func newTestProviders(p ...testProvider) *testProviders {
	m := make(map[string]domain.Provider)
	for _, prov := range p {
		m[prov.name] = prov
	}
	return &testProviders{providers: m}
}

func (t *testProviders) Get(name string) (domain.Provider, bool) {
	p, ok := t.providers[name]
	return p, ok
}

type testProvider struct {
	name    string
	prepare func(ctx context.Context, req domain.Request) (domain.Workspace, error)
}

func (t testProvider) Prepare(ctx context.Context, req domain.Request) (domain.Workspace, error) {
	if t.prepare != nil {
		return t.prepare(ctx, req)
	}
	return domain.Workspace{Repo: req.Repo, BaseSHA: req.BaseSHA, HeadSHA: req.HeadSHA}, nil
}

type blockingRunner struct{}

func (blockingRunner) Run(_ context.Context, w domain.Workspace) (domain.Result, error) {
	return domain.Result{
		Summary:  "review completed",
		Findings: []domain.Finding{},
		OCR:      domain.OCRMetadata{Engine: "test", Status: "success", FilesReviewed: 1},
	}, nil
}

type failingRunner struct {
	err error
}

func (f failingRunner) Run(_ context.Context, _ domain.Workspace) (domain.Result, error) {
	return domain.Result{}, f.err
}

type staticRunner struct {
	result domain.Result
}

func (s staticRunner) Run(_ context.Context, _ domain.Workspace) (domain.Result, error) {
	return s.result, nil
}

type mockExecutor struct {
	runner domain.Runner
	repo   reviewUpdater
}

type reviewUpdater interface {
	updateReviewStatus(id, status string)
}

func (e mockExecutor) Execute(ctx context.Context, id string, req domain.Request, provider domain.Provider, _ progress.ProgressReporter) (domain.Response, error) {
	workspace, err := provider.Prepare(ctx, req)
	if err != nil {
		if e.repo != nil {
			e.repo.updateReviewStatus(id, "failed")
		}
		return domain.Response{ReviewID: id, Status: "failed", Error: err.Error()}, err
	}

	result, err := e.runner.Run(ctx, workspace)
	if err != nil {
		if e.repo != nil {
			e.repo.updateReviewStatus(id, "failed")
		}
		return domain.Response{ReviewID: id, Status: "failed", Error: err.Error(), Gate: domain.ErrorGate("review execution failed")}, err
	}

	result.Scope = workspace.Scope
	report := review.BuildReport(&result)
	analysis := review.BuildAnalysis(result)

	gate := policy.NewEngine(policy.Config{FailOn: "critical"}).Evaluate(toPolicyFindings(result.Findings))

	resp := domain.Response{
		ReviewID: id,
		Status:   "success",
		Summary:  result.Summary,
		Report:   report,
		Analysis: analysis,
		Findings: result.Findings,
		Gate:     domain.CompletedGate(gate.Passed, gate.Reason),
	}

	if e.repo != nil {
		e.repo.updateReviewStatus(id, "success")
	}
	return resp, nil
}

func (m *mockReviewRepo) updateReviewStatus(id, status string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, r := range m.reviews {
		if r.ID == id {
			r.Status = status
			break
		}
	}
}

func (m *mockReviewRepoWithActive) updateReviewStatus(id, status string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, r := range m.reviews {
		if r.ID == id {
			r.Status = status
			break
		}
	}
}

func toPolicyFindings(findings []domain.Finding) []policy.Finding {
	result := make([]policy.Finding, len(findings))
	for i, f := range findings {
		result[i] = policy.Finding{Severity: f.Severity, Path: f.Path}
	}
	return result
}

func newTestService(repo *mockReviewRepo, runner domain.Runner) *review.Service {
	return newTestServiceWithPolicy(repo, runner, policy.Config{FailOn: "critical"})
}

func newTestServiceWithPolicy(repo *mockReviewRepo, runner domain.Runner, policyCfg policy.Config) *review.Service {
	return review.NewService(review.ServiceConfig{
		ReviewRepo:      repo,
		OCRSettingsRepo: &mockOCRSettingsRepo{},
		ProjectRepo:     &mockProjectPolicyRepo{},
		Policy:          policy.NewEngine(policyCfg),
		Providers:       newTestProviders(testProvider{name: "github"}),
		Runner:          runner,
		Executor:        mockExecutor{runner: runner, repo: repo},
	})
}

func validRequest() domain.Request {
	return domain.Request{
		Provider: "github",
		Repo:     "acme/app",
		BaseSHA:  "a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2",
		HeadSHA:  "f6e5d4c3b2a1f6e5d4c3b2a1f6e5d4c3b2a1f6e5",
		Wait:     true,
	}
}

func startWorkers(svc *review.Service, concurrency int) context.CancelFunc {
	ctx, cancel := context.WithCancel(context.Background())
	svc.RunWorkers(ctx, concurrency, 50*time.Millisecond, 5*time.Minute)
	return cancel
}
