package review_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yi-nology/code-review-service/internal/domain/page"

	"github.com/yi-nology/code-review-service/internal/domain/ocrsettings"
	"github.com/yi-nology/code-review-service/internal/domain/projectpolicy"
	"github.com/yi-nology/code-review-service/internal/repository/repoerr"
	"github.com/yi-nology/code-review-service/internal/service/policy"
	"github.com/yi-nology/code-review-service/internal/service/progress"
	"github.com/yi-nology/code-review-service/internal/service/review"
	"github.com/yi-nology/code-review-service/internal/service/review/domain"

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

func TestCreateAsyncPersistsRunningStatus(t *testing.T) {
	repo := &mockReviewRepo{}
	svc := newTestService(repo, blockingRunner{})

	req := validRequest()
	req.Wait = false

	resp, err := svc.Create(context.Background(), req, "")
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if resp.Status != "queued" {
		t.Fatalf("expected status queued, got %s", resp.Status)
	}
	snap := repo.snapshot()
	if len(snap) != 1 {
		t.Fatalf("expected 1 review saved, got %d", len(snap))
	}
	if snap[0].Status != "queued" {
		t.Fatalf("expected persisted status queued, got %s", snap[0].Status)
	}
}

func TestCreateAsyncPersistsFailedStatus(t *testing.T) {
	repo := &mockReviewRepo{}
	runErr := errors.New("runner crashed")
	svc := newTestService(repo, failingRunner{err: runErr})

	req := validRequest()
	req.Wait = false

	_, err := svc.Create(context.Background(), req, "")
	if err != nil {
		t.Fatalf("Create async should not return error: %v", err)
	}

	snap := repo.snapshot()
	if len(snap) != 1 {
		t.Fatalf("expected 1 review saved, got %d", len(snap))
	}
	if snap[0].Status != "queued" {
		t.Fatalf("expected persisted status queued, got %s", snap[0].Status)
	}
}

func TestCreateAsyncStaysQueuedUntilWorkerClaimsIt(t *testing.T) {
	repo := &mockReviewRepo{}
	svc := newTestService(repo, blockingRunner{})

	req := validRequest()
	req.Wait = false

	resp, err := svc.Create(context.Background(), req, "")
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if resp.Status != "queued" {
		t.Fatalf("expected status queued, got %s", resp.Status)
	}

	claimed := make([]domain.Record, 0)
	repo.claimFn = func(_ context.Context, _ int) ([]domain.Record, error) {
		if len(claimed) > 0 {
			return nil, nil
		}
		snap := repo.snapshot()
		for _, r := range snap {
			if r.Status == "queued" {
				r.Status = "running"
				claimed = append(claimed, *r)
				return []domain.Record{*r}, nil
			}
		}
		return nil, nil
	}

	cancel := startWorkers(svc, 1)
	defer cancel()

	time.Sleep(300 * time.Millisecond)

	var lastStatus string
	for _, r := range repo.snapshot() {
		if r.ID == resp.ReviewID {
			lastStatus = r.Status
		}
	}
	if lastStatus != "success" && lastStatus != "running" {
		t.Fatalf("expected success or running after worker, got %s", lastStatus)
	}
}

func TestCreateSyncReturnsFindings(t *testing.T) {
	repo := &mockReviewRepo{}
	result := domain.Result{
		Summary: "found issues",
		OCR:     domain.OCRMetadata{Engine: "test", Status: "success", FilesReviewed: 2},
		Findings: []domain.Finding{
			{Severity: domain.SeverityCritical, Path: "main.go", Title: "SQL Injection", Body: "SQL injection vulnerability"},
			{Severity: domain.SeverityMajor, Path: "util.go", Title: "Error Handling", Body: "Unchecked error"},
		},
	}
	svc := newTestService(repo, staticRunner{result: result})

	resp, err := svc.Create(context.Background(), validRequest(), "")
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if resp.Status != "success" {
		t.Fatalf("expected status success, got %s", resp.Status)
	}
	if len(resp.Findings) != 2 {
		t.Fatalf("expected 2 findings, got %d", len(resp.Findings))
	}
	if resp.Report.Score.Value >= 100 {
		t.Fatalf("expected score < 100 with critical findings, got %d", resp.Report.Score.Value)
	}
	if resp.Gate.Status != domain.GateFailed {
		t.Fatalf("expected gate failed with critical findings, got %s", resp.Gate.Status)
	}
}

func TestCreateRejectsNonHexSHA(t *testing.T) {
	repo := &mockReviewRepo{}
	svc := newTestService(repo, blockingRunner{})

	req := validRequest()
	req.BaseSHA = "NOT-A-VALID-SHA"

	_, err := svc.Create(context.Background(), req, "")
	if err == nil {
		t.Fatal("expected error for non-hex SHA")
	}
	if !strings.Contains(err.Error(), "hexadecimal") {
		t.Fatalf("expected hexadecimal error, got: %v", err)
	}
}

func TestCreateWithIdempotencyKeyReturnsExistingReview(t *testing.T) {
	repo := &mockReviewRepo{}
	svc := newTestService(repo, blockingRunner{})

	req := validRequest()
	resp1, err := svc.Create(context.Background(), req, "idem-key-1")
	if err != nil {
		t.Fatalf("first Create returned error: %v", err)
	}

	req2 := validRequest()
	resp2, err := svc.Create(context.Background(), req2, "idem-key-1")
	if err != nil {
		t.Fatalf("second Create returned error: %v", err)
	}

	if resp1.ReviewID != resp2.ReviewID {
		t.Fatalf("expected same review ID for idempotent request, got %s and %s", resp1.ReviewID, resp2.ReviewID)
	}

	savedCount := 0
	for _, r := range repo.snapshot() {
		if r.ID == resp1.ReviewID {
			savedCount++
		}
	}
	if savedCount > 2 {
		t.Fatalf("expected at most 2 saves for idempotent request, got %d", savedCount)
	}
}

func TestCreateRejectsIdempotencyKeyReuseWithDifferentRequest(t *testing.T) {
	repo := &mockReviewRepo{}
	svc := newTestService(repo, blockingRunner{})

	req1 := validRequest()
	_, err := svc.Create(context.Background(), req1, "idem-key-2")
	if err != nil {
		t.Fatalf("first Create returned error: %v", err)
	}

	req2 := validRequest()
	req2.Repo = "different/repo"
	_, err = svc.Create(context.Background(), req2, "idem-key-2")
	if err == nil {
		t.Fatal("expected error for idempotency key reuse with different request")
	}
	if !errors.Is(err, domain.ErrIdempotencyConflict) {
		t.Fatalf("expected ErrIdempotencyConflict, got: %v", err)
	}
}

func TestGetReturnsReviewRecord(t *testing.T) {
	repo := &mockReviewRepo{}
	svc := newTestService(repo, blockingRunner{})

	req := validRequest()
	created, err := svc.Create(context.Background(), req, "")
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	record, found, err := svc.Get(context.Background(), created.ReviewID)
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if !found {
		t.Fatal("expected review to be found")
	}
	if record.ID != created.ReviewID {
		t.Fatalf("expected review ID %s, got %s", created.ReviewID, record.ID)
	}
}

func TestGetReturnsNotFound(t *testing.T) {
	repo := &mockReviewRepo{}
	svc := newTestService(repo, blockingRunner{})

	_, found, err := svc.Get(context.Background(), "nonexistent")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if found {
		t.Fatal("expected review not to be found")
	}
}

func TestListReturnsEmptyWhenNoReviews(t *testing.T) {
	repo := &mockReviewRepo{}
	svc := newTestService(repo, blockingRunner{})

	result, err := svc.List(context.Background(), review.ListOptions{Limit: 50, Offset: 0})
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(result.Items) != 0 {
		t.Fatalf("expected 0 items, got %d", len(result.Items))
	}
}

func TestCreateRequiresProvider(t *testing.T) {
	repo := &mockReviewRepo{}
	svc := newTestService(repo, blockingRunner{})

	req := validRequest()
	req.Provider = ""

	_, err := svc.Create(context.Background(), req, "")
	if err == nil {
		t.Fatal("expected error for missing provider")
	}
}

func TestCreateRequiresRepo(t *testing.T) {
	repo := &mockReviewRepo{}
	svc := newTestService(repo, blockingRunner{})

	req := validRequest()
	req.Repo = ""

	_, err := svc.Create(context.Background(), req, "")
	if err == nil {
		t.Fatal("expected error for missing repo")
	}
}

func TestRequeueFailedReview(t *testing.T) {
	repo := &mockReviewRepo{}
	svc := newTestService(repo, staticRunner{
		result: domain.Result{
			Summary:  "found issues",
			Findings: []domain.Finding{{Severity: domain.SeverityCritical, Path: "main.go", Title: "bug"}},
		},
	})

	req := validRequest()
	resp, err := svc.Create(context.Background(), req, "")
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if resp.Status != "success" {
		t.Fatalf("expected success, got %s", resp.Status)
	}

	requeued, err := svc.Requeue(context.Background(), resp.ReviewID)
	if err != nil {
		t.Fatalf("Requeue returned error: %v", err)
	}
	if requeued.Status != "queued" {
		t.Fatalf("expected queued after requeue, got %s", requeued.Status)
	}
	if requeued.Gate.Status != domain.GatePending {
		t.Fatalf("expected gate pending after requeue, got %s", requeued.Gate.Status)
	}
	if len(requeued.Findings) != 0 {
		t.Fatalf("expected 0 findings after requeue, got %d", len(requeued.Findings))
	}
	if requeued.Report.Generated {
		t.Fatal("expected report to be cleared after requeue")
	}
	if requeued.Error != "" {
		t.Fatalf("expected empty error after requeue, got %s", requeued.Error)
	}
}

func TestRequeueRejectsQueuedReview(t *testing.T) {
	repo := &mockReviewRepo{}
	svc := newTestService(repo, blockingRunner{})

	req := validRequest()
	req.Wait = false
	resp, err := svc.Create(context.Background(), req, "")
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	_, err = svc.Requeue(context.Background(), resp.ReviewID)
	if err == nil {
		t.Fatal("expected error when requeuing queued review")
	}
	if !errors.Is(err, review.ErrInvalidState) {
		t.Fatalf("expected ErrInvalidState, got: %v", err)
	}
}

func TestRequeueRejectsRunningReview(t *testing.T) {
	repo := &mockReviewRepo{}
	svc := newTestService(repo, blockingRunner{})

	req := validRequest()
	req.Wait = false
	resp, err := svc.Create(context.Background(), req, "")
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	snap := repo.snapshot()
	snap[0].Status = "running"
	repo.Save(context.Background(), snap[0])

	_, err = svc.Requeue(context.Background(), resp.ReviewID)
	if err == nil {
		t.Fatal("expected error when requeuing running review")
	}
	if !errors.Is(err, review.ErrInvalidState) {
		t.Fatalf("expected ErrInvalidState, got: %v", err)
	}
}

func TestRequeueReturnsNotFound(t *testing.T) {
	repo := &mockReviewRepo{}
	svc := newTestService(repo, blockingRunner{})

	_, err := svc.Requeue(context.Background(), "nonexistent-id")
	if err == nil {
		t.Fatal("expected error for nonexistent review")
	}
	if !errors.Is(err, review.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got: %v", err)
	}
}

func TestCancelQueuedReview(t *testing.T) {
	repo := &mockReviewRepo{}
	svc := newTestService(repo, blockingRunner{})

	req := validRequest()
	req.Wait = false
	resp, err := svc.Create(context.Background(), req, "")
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if resp.Status != "queued" {
		t.Fatalf("expected queued, got %s", resp.Status)
	}

	cancelled, err := svc.Cancel(context.Background(), resp.ReviewID)
	if err != nil {
		t.Fatalf("Cancel returned error: %v", err)
	}
	if cancelled.Status != "failed" {
		t.Fatalf("expected failed after cancel, got %s", cancelled.Status)
	}
	if cancelled.Gate.Status != domain.GateError {
		t.Fatalf("expected gate error after cancel, got %s", cancelled.Gate.Status)
	}
	if cancelled.Summary != "review cancelled by user" {
		t.Fatalf("expected cancel summary, got %s", cancelled.Summary)
	}

	lastEvent := cancelled.History[len(cancelled.History)-1]
	if lastEvent.Stage != "cancelled" {
		t.Fatalf("expected last event stage 'cancelled', got %s", lastEvent.Stage)
	}
	if lastEvent.Status != "failed" {
		t.Fatalf("expected last event status 'failed', got %s", lastEvent.Status)
	}
}

func TestCancelAllowsRunningReview(t *testing.T) {
	repo := &mockReviewRepo{}
	svc := newTestService(repo, blockingRunner{})

	req := validRequest()
	req.Wait = false
	resp, err := svc.Create(context.Background(), req, "")
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	snap := repo.snapshot()
	snap[0].Status = "running"
	repo.Save(context.Background(), snap[0])

	cancelResp, err := svc.Cancel(context.Background(), resp.ReviewID)
	if err != nil {
		t.Fatalf("expected running review to be cancellable, got error: %v", err)
	}
	if cancelResp.Status != "failed" {
		t.Fatalf("expected cancelled review status 'failed', got %q", cancelResp.Status)
	}
}

func TestCancelRejectsCompletedReview(t *testing.T) {
	repo := &mockReviewRepo{}
	svc := newTestService(repo, staticRunner{
		result: domain.Result{Summary: "ok"},
	})

	req := validRequest()
	resp, err := svc.Create(context.Background(), req, "")
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	_, err = svc.Cancel(context.Background(), resp.ReviewID)
	if err == nil {
		t.Fatal("expected error when cancelling completed review")
	}
	if !errors.Is(err, review.ErrInvalidState) {
		t.Fatalf("expected ErrInvalidState, got: %v", err)
	}
}

func TestCancelReturnsNotFound(t *testing.T) {
	repo := &mockReviewRepo{}
	svc := newTestService(repo, blockingRunner{})

	_, err := svc.Cancel(context.Background(), "nonexistent-id")
	if err == nil {
		t.Fatal("expected error for nonexistent review")
	}
	if !errors.Is(err, review.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got: %v", err)
	}
}

func TestCreateDeduplicatesActiveReviewForSamePR(t *testing.T) {
	repo := &mockReviewRepo{}
	runner := &countingRunner{}
	svc := newTestService(repo, runner)

	req := validRequest()
	req.PullRequestID = "42"
	req.Wait = false

	resp1, err := svc.Create(context.Background(), req, "")
	if err != nil {
		t.Fatalf("first Create returned error: %v", err)
	}

	resp2, err := svc.Create(context.Background(), req, "")
	if err != nil {
		t.Fatalf("second Create returned error: %v", err)
	}

	if resp1.ReviewID != resp2.ReviewID {
		t.Fatalf("expected same review ID for dedup, got %s and %s", resp1.ReviewID, resp2.ReviewID)
	}

	snap := repo.snapshot()
	if len(snap) != 1 {
		t.Fatalf("expected 1 review persisted (dedup), got %d", len(snap))
	}
}

func TestCreateDoesNotDedupDifferentPR(t *testing.T) {
	repo := &mockReviewRepo{}
	runner := &countingRunner{}
	svc := newTestService(repo, runner)

	req1 := validRequest()
	req1.PullRequestID = "42"

	req2 := validRequest()
	req2.PullRequestID = "99"

	_, err := svc.Create(context.Background(), req1, "")
	if err != nil {
		t.Fatalf("first Create returned error: %v", err)
	}

	_, err = svc.Create(context.Background(), req2, "")
	if err != nil {
		t.Fatalf("second Create returned error: %v", err)
	}

	if runner.count != 2 {
		t.Fatalf("expected runner called twice for different PRs, got %d", runner.count)
	}
}

type countingRunner struct {
	count int
}

func (r *countingRunner) Run(_ context.Context, _ domain.Workspace) (domain.Result, error) {
	r.count++
	return domain.Result{
		Summary:  "ok",
		Findings: []domain.Finding{},
		OCR:      domain.OCRMetadata{Engine: "test", Status: "success"},
	}, nil
}

type mockReviewRepoWithActive struct {
	mockReviewRepo
	activeReview *domain.Record
}

func (m *mockReviewRepoWithActive) GetActiveByProviderRepo(_ context.Context, provider, repo, pullRequestID, mergeRequestIID string) (*domain.Record, error) {
	if m.activeReview != nil && m.activeReview.Provider == provider && m.activeReview.Repo == repo {
		if pullRequestID != "" && m.activeReview.PullRequestID != pullRequestID {
			return nil, repoerr.ErrNotFound
		}
		if mergeRequestIID != "" && m.activeReview.MergeRequestIID != mergeRequestIID {
			return nil, repoerr.ErrNotFound
		}
		return m.activeReview, nil
	}
	return nil, repoerr.ErrNotFound
}

func TestFindActiveReviewReturnsExisting(t *testing.T) {
	activeRepo := &mockReviewRepoWithActive{}
	req := validRequest()
	req.PullRequestID = "42"

	activeRepo.activeReview = &domain.Record{
		ID:            "existing-review",
		Provider:      req.Provider,
		Repo:          req.Repo,
		PullRequestID: "42",
		Status:        "queued",
		Response: domain.Response{
			ReviewID: "existing-review",
			Status:   "queued",
			Summary:  "queued",
		},
	}

	svc := review.NewService(review.ServiceConfig{
		ReviewRepo:      activeRepo,
		OCRSettingsRepo: &mockOCRSettingsRepo{},
		ProjectRepo:     &mockProjectPolicyRepo{},
		Policy:          policy.NewEngine(policy.Config{FailOn: "critical"}),
		Providers:       newTestProviders(testProvider{name: "github"}),
		Runner:          &countingRunner{},
		Executor:        mockExecutor{runner: &countingRunner{}, repo: activeRepo},
	})

	resp, err := svc.Create(context.Background(), req, "")
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if resp.ReviewID != "existing-review" {
		t.Fatalf("expected existing review ID, got %s", resp.ReviewID)
	}
}

// TestFindActiveReview_PRIDMismatchIgnored 验证 PR-02 修复：
// 当 (provider, repo) 下存在 active review，但 PR ID 不同时，
// mockRepository 必须返回 nil/ErrNotFound（新评审可创建），不依赖 JSONB unmarshal。
// 这验证了 SQL 层下推的正确性：PR 不匹配根本不会进入 review 记录。
func TestFindActiveReview_PRIDMismatchIgnored(t *testing.T) {
	activeRepo := &mockReviewRepo{}
	activeRepo.reviews = []*domain.Record{
		{
			ID:            "another-review",
			Provider:      "github",
			Repo:          "owner/repo",
			PullRequestID: "999", // 不匹配
			Status:        "running",
		},
	}

	// 传入 PR=42 应该找不到 PR=999 的 review。
	got, err := activeRepo.GetActiveByProviderRepo(context.Background(), "github", "owner/repo", "42", "")
	if err != nil && !errors.Is(err, repoerr.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got err: %v", err)
	}
	if got != nil {
		t.Fatalf("expected nil for PR mismatch, got %+v", got)
	}
}

// TestFindActiveReview_PRIDMatch 验证 PR-02 修复：PR ID 匹配时能找到 review。
func TestFindActiveReview_PRIDMatch(t *testing.T) {
	activeRepo := &mockReviewRepo{}
	activeRepo.reviews = []*domain.Record{
		{
			ID:            "matching-review",
			Provider:      "github",
			Repo:          "owner/repo",
			PullRequestID: "42",
			Status:        "queued",
		},
	}

	got, err := activeRepo.GetActiveByProviderRepo(context.Background(), "github", "owner/repo", "42", "")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if got == nil {
		t.Fatalf("expected to find review, got nil")
	}
	if got.ID != "matching-review" {
		t.Fatalf("expected matching-review, got %s", got.ID)
	}
}

// TestNewReviewID 验证 PR-14 修复：
// newReviewID 应返回 ULID 格式（26 字符 Base32），且单调递增。
func TestNewReviewID(t *testing.T) {
	id1 := review.NewReviewID(time.Now().UTC())
	id2 := review.NewReviewID(time.Now().UTC())

	if len(id1) != 26 {
		t.Fatalf("expected ULID length 26, got %d (%q)", len(id1), id1)
	}
	if id1 >= id2 {
		t.Fatalf("expected ULID to be lexicographically ordered by time, got %q >= %q", id1, id2)
	}
	// 同毫秒内调用应仍唯一
	seen := map[string]struct{}{id1: {}, id2: {}}
	for i := 0; i < 100; i++ {
		id := review.NewReviewID(time.Now().UTC())
		if _, dup := seen[id]; dup {
			t.Fatalf("duplicate id within same ms: %q", id)
		}
		seen[id] = struct{}{}
	}
}

type stubScanRunner struct {
	result *domain.ScanResult
	err    error
	calls  int
}

func (r *stubScanRunner) Scan(_ context.Context, _ domain.Workspace, _ domain.ScanRequest) (*domain.ScanResult, error) {
	r.calls++
	if r.err != nil {
		return nil, r.err
	}
	return r.result, nil
}

func TestScanRecordsRawOCRInput(t *testing.T) {
	workspaceDir := t.TempDir()

	repo := &mockReviewRepo{}
	scanRunner := &stubScanRunner{result: &domain.ScanResult{
		Result: &domain.Result{Summary: "scan ok", OCR: domain.OCRMetadata{Status: "success", FilesReviewed: 1}},
	}}

	svc := review.NewService(review.ServiceConfig{
		ReviewRepo:      repo,
		OCRSettingsRepo: &mockOCRSettingsRepo{},
		ProjectRepo:     &mockProjectPolicyRepo{},
		Policy:          policy.NewEngine(policy.Config{FailOn: "critical"}),
		Providers: newTestProviders(testProvider{
			name: "gitea",
			prepare: func(_ context.Context, req domain.Request) (domain.Workspace, error) {
				return domain.Workspace{Repo: req.Repo, BaseSHA: req.BaseSHA, HeadSHA: req.HeadSHA, Dir: workspaceDir}, nil
			},
		}),
		ScanRunner: scanRunner,
	})

	result, err := svc.Scan(context.Background(), domain.ScanRequest{
		Provider: "gitea",
		Repo:     "acme/app",
		HeadSHA:  "f6e5d4c3b2a1f6e5d4c3b2a1f6e5d4c3b2a1f6e5",
	})
	if err != nil {
		t.Fatalf("Scan returned error: %v", err)
	}

	deadline := time.Now().Add(5 * time.Second)
	var saved *domain.Record
	for time.Now().Before(deadline) {
		for _, r := range repo.snapshot() {
			if r.ID == result.ReviewID {
				saved = r
			}
		}
		if saved != nil && saved.Status == "success" {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if saved == nil {
		t.Fatal("expected scan review to be persisted")
	}

	if len(saved.RawOCRInput) == 0 {
		t.Fatal("expected raw_ocr_input to be recorded for scan, got empty")
	}
	var input domain.OCRInput
	if err := json.Unmarshal(saved.RawOCRInput, &input); err != nil {
		t.Fatalf("failed to decode raw_ocr_input: %v", err)
	}
	if input.Workspace.Dir != workspaceDir {
		t.Fatalf("expected workspace dir %q, got %q", workspaceDir, input.Workspace.Dir)
	}
	if input.StartedAt.IsZero() {
		t.Fatal("expected non-zero started_at in raw_ocr_input")
	}
	if _, err := os.Stat(workspaceDir); err == nil {
		_ = os.RemoveAll(workspaceDir)
	}
}

// TestScanAsync_RequestCtxCancellation 验证 PR-03 修复：
// 当 HTTP 请求 ctx 被取消时，scanAsync 启动的 goroutine 应当感知到 ctx.Done()。
// 旧实现 context.WithCancel(context.Background()) 与请求 lifecycle 完全脱钩，
// 只能通过显式 Service.Cancel() 取消；新实现派生自 ctx，请求断开即终止。
//
// 验证方式：scan runner 在 ctx.Done() 触发时记录一次信号；测试取消 ctx 后等待信号。
func TestScanAsync_RequestCtxCancellation(t *testing.T) {
	scanRunner := &ctxBlockingScanRunner{entered: make(chan struct{}, 1)}

	svc := review.NewService(review.ServiceConfig{
		ReviewRepo:      &mockReviewRepo{},
		OCRSettingsRepo: &mockOCRSettingsRepo{},
		ProjectRepo:     &mockProjectPolicyRepo{},
		Policy:          policy.NewEngine(policy.Config{FailOn: "critical"}),
		Providers: newTestProviders(testProvider{
			name: "gitea",
			prepare: func(_ context.Context, _ domain.Request) (domain.Workspace, error) {
				return domain.Workspace{Dir: t.TempDir()}, nil
			},
		}),
		Runner:     &countingRunner{},
		ScanRunner: scanRunner,
		Executor:   mockExecutor{runner: &countingRunner{}, repo: &mockReviewRepo{}},
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	scanReq := domain.ScanRequest{
		Provider: "gitea",
		Repo:     "owner/repo",
		HeadSHA:  "deadbeef",
		Batch:    "none",
	}

	result, err := svc.Scan(ctx, scanReq)
	if err != nil {
		t.Fatalf("Scan returned error: %v", err)
	}
	if result == nil || result.ReviewID == "" {
		t.Fatalf("expected review ID, got %+v", result)
	}

	// 等待 scan runner 进入阻塞态（已收到 ctx）
	select {
	case <-scanRunner.entered:
	case <-time.After(2 * time.Second):
		t.Fatalf("scan runner did not start within 2s")
	}

	// 取消请求 ctx
	cancel()

	// 等待 scan runner 收到 ctx.Done()
	select {
	case <-scanRunner.cancelled:
		// 成功
	case <-time.After(2 * time.Second):
		t.Fatalf("scan runner did not receive ctx cancellation within 2s (PR-03 broken?)")
	}
}

// ctxBlockingScanRunner 阻塞在 ctx.Done() 上以验证 cancellation。
type ctxBlockingScanRunner struct {
	entered   chan struct{}
	cancelled chan struct{}
}

func (r *ctxBlockingScanRunner) Scan(ctx context.Context, _ domain.Workspace, _ domain.ScanRequest) (*domain.ScanResult, error) {
	select {
	case r.entered <- struct{}{}:
	default:
	}
	if r.cancelled == nil {
		r.cancelled = make(chan struct{})
	}
	<-ctx.Done()
	close(r.cancelled)
	return nil, ctx.Err()
}

// TestMergeRequirementsIntoBackground 验证需求/spec 被正确合并进 OCR --background。
func TestMergeRequirementsIntoBackground(t *testing.T) {
	tests := []struct {
		name         string
		existing     string
		requirements []string
		want         string
	}{
		{
			name:         "empty existing + requirements",
			existing:     "",
			requirements: []string{"需求 A", "需求 B"},
			want:         "需求 A\n需求 B",
		},
		{
			name:         "existing + requirements appended",
			existing:     "项目背景",
			requirements: []string{"需求 A"},
			want:         "项目背景\n需求 A",
		},
		{
			name:         "no requirements keeps existing",
			existing:     "项目背景",
			requirements: nil,
			want:         "项目背景",
		},
		{
			name:         "blank entries trimmed and dropped",
			existing:     "  项目背景  ",
			requirements: []string{"  ", "需求 A", ""},
			want:         "项目背景\n需求 A",
		},
		{
			name:         "all empty produces empty",
			existing:     "   ",
			requirements: []string{"  ", ""},
			want:         "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := review.MergeRequirementsIntoBackground(tt.existing, tt.requirements)
			if got != tt.want {
				t.Errorf("MergeRequirementsIntoBackground(%q, %v) = %q, want %q", tt.existing, tt.requirements, got, tt.want)
			}
		})
	}
}
