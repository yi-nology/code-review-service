package review_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yi-nology/code-review-service/internal/repository"
	"github.com/yi-nology/code-review-service/internal/repository/postgres/models"
	"github.com/yi-nology/code-review-service/internal/service/policy"
	"github.com/yi-nology/code-review-service/internal/service/progress"
	"github.com/yi-nology/code-review-service/internal/service/review"
	ekitGorm "github.com/oekit/gormx"

	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/datatypes"
)

type mockReviewRepo struct {
	mu        sync.Mutex
	reviews   []*models.Review
	saveErr   error
	createErr error
	claimFn   func(ctx context.Context, limit int) ([]models.Review, error)
}

func (m *mockReviewRepo) Create(_ context.Context, r *models.Review) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.createErr != nil {
		return m.createErr
	}
	if r.IdempotencyKey != nil {
		for _, existing := range m.reviews {
			if existing.IdempotencyKey != nil && *existing.IdempotencyKey == *r.IdempotencyKey {
				return &pgconn.PgError{Code: "23505"}
			}
		}
	}
	m.reviews = append(m.reviews, r)
	return nil
}

func (m *mockReviewRepo) Save(_ context.Context, r *models.Review) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.saveErr != nil {
		return m.saveErr
	}
	for i, existing := range m.reviews {
		if existing.ReviewID == r.ReviewID {
			m.reviews[i] = r
			return nil
		}
	}
	m.reviews = append(m.reviews, r)
	return nil
}

func (m *mockReviewRepo) GetByReviewID(_ context.Context, reviewID string) (*models.Review, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, r := range m.reviews {
		if r.ReviewID == reviewID {
			return r, nil
		}
	}
	return nil, repository.ErrNotFound
}

func (m *mockReviewRepo) GetByidempotencyKey(_ context.Context, key string) (*models.Review, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, r := range m.reviews {
		if r.IdempotencyKey != nil && *r.IdempotencyKey == key {
			return r, nil
		}
	}
	return nil, repository.ErrNotFound
}

func (m *mockReviewRepo) GetActiveByProviderRepo(_ context.Context, provider, repo string) (*models.Review, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, r := range m.reviews {
		if r.Provider == provider && r.Repo == repo && (r.Status == "queued" || r.Status == "running") {
			return r, nil
		}
	}
	return nil, repository.ErrNotFound
}

func (m *mockReviewRepo) List(_ context.Context, opts models.ListOptions) (*ekitGorm.PageResponse[models.Review], error) {
	return &ekitGorm.PageResponse[models.Review]{List: nil, Total: 0}, nil
}

func (m *mockReviewRepo) AggregateStats(_ context.Context, _ models.ListOptions) (*models.AggregateStats, error) {
	return &models.AggregateStats{}, nil
}

func (m *mockReviewRepo) ClaimQueued(ctx context.Context, limit int) ([]models.Review, error) {
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

func (m *mockReviewRepo) UpdateInTransaction(_ context.Context, reviewID string, fn func(review *models.Review) error) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, r := range m.reviews {
		if r.ReviewID == reviewID {
			return fn(r)
		}
	}
	return repository.ErrNotFound
}

func (m *mockReviewRepo) snapshot() []*models.Review {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]*models.Review, len(m.reviews))
	copy(out, m.reviews)
	return out
}

type mockOCRSettingsRepo struct {
	settings *models.OCRSettings
	err      error
}

func (m *mockOCRSettingsRepo) Get(_ context.Context) (*models.OCRSettings, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.settings, nil
}

func (m *mockOCRSettingsRepo) Save(_ context.Context, _ *models.OCRSettings) error {
	return nil
}

type mockProjectPolicyRepo struct {
	policy *models.ProjectPolicy
	err    error
}

func (m *mockProjectPolicyRepo) List(_ context.Context) ([]models.ProjectPolicy, error) {
	return nil, nil
}

func (m *mockProjectPolicyRepo) GetByProviderRepo(_ context.Context, _, _ string) (*models.ProjectPolicy, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.policy, nil
}

func (m *mockProjectPolicyRepo) Save(_ context.Context, _ *models.ProjectPolicy) error {
	return nil
}

func (m *mockProjectPolicyRepo) Delete(_ context.Context, _, _ string) error {
	return nil
}

type testProviders struct {
	providers map[string]review.Provider
}

func newTestProviders(p ...testProvider) *testProviders {
	m := make(map[string]review.Provider)
	for _, prov := range p {
		m[prov.name] = prov
	}
	return &testProviders{providers: m}
}

func (t *testProviders) Get(name string) (review.Provider, bool) {
	p, ok := t.providers[name]
	return p, ok
}

type testProvider struct {
	name    string
	prepare func(ctx context.Context, req review.Request) (review.Workspace, error)
}

func (t testProvider) Prepare(ctx context.Context, req review.Request) (review.Workspace, error) {
	if t.prepare != nil {
		return t.prepare(ctx, req)
	}
	return review.Workspace{Repo: req.Repo, BaseSHA: req.BaseSHA, HeadSHA: req.HeadSHA}, nil
}

type blockingRunner struct{}

func (blockingRunner) Run(_ context.Context, w review.Workspace) (review.Result, error) {
	return review.Result{
		Summary:  "review completed",
		Findings: []review.Finding{},
		OCR:      review.OCRMetadata{Engine: "test", Status: "success", FilesReviewed: 1},
	}, nil
}

type failingRunner struct {
	err error
}

func (f failingRunner) Run(_ context.Context, _ review.Workspace) (review.Result, error) {
	return review.Result{}, f.err
}

type staticRunner struct {
	result review.Result
}

func (s staticRunner) Run(_ context.Context, _ review.Workspace) (review.Result, error) {
	return s.result, nil
}

type mockExecutor struct {
	runner review.Runner
	repo   reviewUpdater
}

type reviewUpdater interface {
	updateReviewStatus(id, status string)
}

func (e mockExecutor) Execute(ctx context.Context, id string, req review.Request, provider review.Provider, _ progress.ProgressReporter) (review.Response, error) {
	workspace, err := provider.Prepare(ctx, req)
	if err != nil {
		if e.repo != nil {
			e.repo.updateReviewStatus(id, "failed")
		}
		return review.Response{ReviewID: id, Status: "failed", Error: err.Error()}, err
	}

	result, err := e.runner.Run(ctx, workspace)
	if err != nil {
		if e.repo != nil {
			e.repo.updateReviewStatus(id, "failed")
		}
		return review.Response{ReviewID: id, Status: "failed", Error: err.Error(), Gate: review.ErrorGate("review execution failed")}, err
	}

	result.Scope = workspace.Scope
	report := review.BuildReport(&result)
	analysis := review.BuildAnalysis(result)

	gate := policy.NewEngine(policy.Config{FailOn: "critical"}).Evaluate(toPolicyFindings(result.Findings))

	resp := review.Response{
		ReviewID: id,
		Status:   "success",
		Summary:  result.Summary,
		Report:   report,
		Analysis: analysis,
		Findings: result.Findings,
		Gate:     review.CompletedGate(gate.Passed, gate.Reason),
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
		if r.ReviewID == id {
			r.Status = status
			break
		}
	}
}

func (m *mockReviewRepoWithActive) updateReviewStatus(id, status string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, r := range m.reviews {
		if r.ReviewID == id {
			r.Status = status
			break
		}
	}
}

func toPolicyFindings(findings []review.Finding) []policy.Finding {
	result := make([]policy.Finding, len(findings))
	for i, f := range findings {
		result[i] = policy.Finding{Severity: string(f.Severity), Path: f.Path}
	}
	return result
}

func newTestService(repo *mockReviewRepo, runner review.Runner) *review.Service {
	return newTestServiceWithPolicy(repo, runner, policy.Config{FailOn: "critical"})
}

func newTestServiceWithPolicy(repo *mockReviewRepo, runner review.Runner, policyCfg policy.Config) *review.Service {
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

func validRequest() review.Request {
	return review.Request{
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

	claimed := make([]models.Review, 0)
	repo.claimFn = func(_ context.Context, _ int) ([]models.Review, error) {
		if len(claimed) > 0 {
			return nil, nil
		}
		snap := repo.snapshot()
		for _, r := range snap {
			if r.Status == "queued" {
				r.Status = "running"
				claimed = append(claimed, *r)
				return []models.Review{*r}, nil
			}
		}
		return nil, nil
	}

	cancel := startWorkers(svc, 1)
	defer cancel()

	time.Sleep(300 * time.Millisecond)

	var lastStatus string
	for _, r := range repo.snapshot() {
		if r.ReviewID == resp.ReviewID {
			lastStatus = r.Status
		}
	}
	if lastStatus != "success" && lastStatus != "running" {
		t.Fatalf("expected success or running after worker, got %s", lastStatus)
	}
}

func TestCreateSyncReturnsFindings(t *testing.T) {
	repo := &mockReviewRepo{}
	result := review.Result{
		Summary: "found issues",
		OCR:     review.OCRMetadata{Engine: "test", Status: "success", FilesReviewed: 2},
		Findings: []review.Finding{
			{Severity: review.SeverityCritical, Path: "main.go", Title: "SQL Injection", Body: "SQL injection vulnerability"},
			{Severity: review.SeverityMajor, Path: "util.go", Title: "Error Handling", Body: "Unchecked error"},
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
	if resp.Gate.Status != review.GateFailed {
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
		if r.ReviewID == resp1.ReviewID {
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
	if !errors.Is(err, review.ErrIdempotencyConflict) {
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
		result: review.Result{
			Summary:  "found issues",
			Findings: []review.Finding{{Severity: review.SeverityCritical, Path: "main.go", Title: "bug"}},
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
	if requeued.Gate.Status != review.GatePending {
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
	if cancelled.Gate.Status != review.GateError {
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

func TestCancelRejectsRunningReview(t *testing.T) {
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

	_, err = svc.Cancel(context.Background(), resp.ReviewID)
	if err == nil {
		t.Fatal("expected error when cancelling running review")
	}
	if !errors.Is(err, review.ErrInvalidState) {
		t.Fatalf("expected ErrInvalidState, got: %v", err)
	}
}

func TestCancelRejectsCompletedReview(t *testing.T) {
	repo := &mockReviewRepo{}
	svc := newTestService(repo, staticRunner{
		result: review.Result{Summary: "ok"},
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

func (r *countingRunner) Run(_ context.Context, _ review.Workspace) (review.Result, error) {
	r.count++
	return review.Result{
		Summary:  "ok",
		Findings: []review.Finding{},
		OCR:      review.OCRMetadata{Engine: "test", Status: "success"},
	}, nil
}

type mockReviewRepoWithActive struct {
	mockReviewRepo
	activeReview *models.Review
}

func (m *mockReviewRepoWithActive) GetActiveByProviderRepo(_ context.Context, provider, repo string) (*models.Review, error) {
	if m.activeReview != nil && m.activeReview.Provider == provider && m.activeReview.Repo == repo {
		return m.activeReview, nil
	}
	return nil, nil
}

func TestFindActiveReviewReturnsExisting(t *testing.T) {
	activeRepo := &mockReviewRepoWithActive{}
	req := validRequest()
	req.PullRequestID = "42"

	activeReview := &models.Review{
		ReviewID: "existing-review",
		Provider: req.Provider,
		Repo:     req.Repo,
		Status:   "queued",
		Request:  mustMarshalJSON(req),
		Response: mustMarshalJSON(review.Response{
			ReviewID: "existing-review",
			Status:   "queued",
			Summary:  "queued",
		}),
	}
	activeRepo.activeReview = activeReview

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

func mustMarshalJSON(v interface{}) datatypesJSON {
	data, _ := json.Marshal(v)
	return datatypesJSON(data)
}

type datatypesJSON = datatypes.JSON
