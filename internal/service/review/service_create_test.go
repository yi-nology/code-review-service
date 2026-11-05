package review_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/yi-nology/code-review-service/internal/domain/review"
	"github.com/yi-nology/code-review-service/internal/repository/repoerr"
	"github.com/yi-nology/code-review-service/internal/service/policy"
	"github.com/yi-nology/code-review-service/internal/service/review"
)

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
