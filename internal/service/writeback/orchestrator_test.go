package writeback

import (
	"context"
	"strings"
	"testing"
	"time"

	wbitem "github.com/yi-nology/code-review-service/internal/domain/writeback"
	"github.com/yi-nology/code-review-service/internal/service/review/domain"
)

type mockWritebackItemRepo struct {
	items []wbitem.Item
}

func (m *mockWritebackItemRepo) Create(_ context.Context, item *wbitem.Item) error {
	for _, existing := range m.items {
		if existing.ReviewID == item.ReviewID && existing.FindingKey == item.FindingKey && existing.Kind == item.Kind {
			return nil
		}
	}
	m.items = append(m.items, *item)
	return nil
}

func (m *mockWritebackItemRepo) UpdateStatus(_ context.Context, reviewID, findingKey, kind, status, externalID, providerURL, lastErr string) error {
	for i := range m.items {
		item := &m.items[i]
		if item.ReviewID == reviewID && item.FindingKey == findingKey && item.Kind == kind {
			item.Status = status
			item.ProviderExternalID = externalID
			item.ProviderURL = providerURL
			item.LastError = lastErr
			return nil
		}
	}
	return nil
}

func (m *mockWritebackItemRepo) IncrementAttempts(_ context.Context, reviewID, findingKey, kind string) error {
	for i := range m.items {
		item := &m.items[i]
		if item.ReviewID == reviewID && item.FindingKey == findingKey && item.Kind == kind {
			item.Attempts++
			return nil
		}
	}
	return nil
}

func (m *mockWritebackItemRepo) ListByReview(context.Context, string) ([]wbitem.Item, error) {
	return m.items, nil
}

func (m *mockWritebackItemRepo) ListRetryable(context.Context, int) ([]wbitem.Item, error) {
	return m.items, nil
}

func (m *mockWritebackItemRepo) ListRecent(context.Context, string, string, int) ([]wbitem.Item, error) {
	return m.items, nil
}

func (m *mockWritebackItemRepo) CountByStatus(context.Context, time.Time) (map[string]int64, error) {
	counts := map[string]int64{}
	for _, item := range m.items {
		counts[item.Status]++
	}
	return counts, nil
}

type mockWritebackAdapter struct {
	result Result
}

func (mockWritebackAdapter) Capabilities(context.Context, ChangeRequest) (Capabilities, error) {
	return Capabilities{BatchReview: true, MultiLineComments: true, Suggestions: true}, nil
}

func (m mockWritebackAdapter) Publish(_ context.Context, review Review) (Result, error) {
	if m.result.ReviewID != "" || len(m.result.Comments) > 0 {
		return m.result, nil
	}
	return defaultWritebackResult(review), nil
}

type capturingWritebackAdapter struct {
	review Review
}

func (capturingWritebackAdapter) Capabilities(context.Context, ChangeRequest) (Capabilities, error) {
	return Capabilities{BatchReview: true, MultiLineComments: true, Suggestions: true}, nil
}

func (m *capturingWritebackAdapter) Publish(_ context.Context, review Review) (Result, error) {
	m.review = review
	return defaultWritebackResult(review), nil
}

func defaultWritebackResult(review Review) Result {
	result := Result{
		ReviewID:   "platform-review-123",
		SummaryID:  "summary-1",
		SummaryURL: "https://example.test/summary-1",
	}
	for i, comment := range review.Comments {
		result.Comments = append(result.Comments, CommentResult{
			Key:        comment.Key,
			ExternalID: "comment-1",
			URL:        "https://example.test/comment-1",
			Inline:     true,
		})
		if i > 0 {
			result.Comments[i].ExternalID = "comment-extra"
		}
	}
	return result
}

func findWritebackItem(items []wbitem.Item, key, kind string) (wbitem.Item, bool) {
	for _, item := range items {
		if item.FindingKey == key && item.Kind == kind {
			return item, true
		}
	}
	return wbitem.Item{}, false
}

func testFindingKey() string {
	return FindingKey("", "a.go", SideNew, 1, "body")
}

// staleThenOKAdapter 第一次 Publish 返回 stale（head 已变），第二次（降级 summary-only）返回成功。
// 用于验证 orchestrator 在 stale 时降级为以当前 head 发布摘要。
type staleThenOKAdapter struct {
	calls   int
	reviews []Review
}

func (staleThenOKAdapter) Capabilities(context.Context, ChangeRequest) (Capabilities, error) {
	return Capabilities{BatchReview: true, MultiLineComments: true, Suggestions: true}, nil
}

func (a *staleThenOKAdapter) Publish(_ context.Context, review Review) (Result, error) {
	a.calls++
	a.reviews = append(a.reviews, review)
	if a.calls == 1 {
		return Result{Stale: true, ExpectedSHA: review.ChangeRequest.HeadSHA, ActualSHA: "newhead"}, nil
	}
	return Result{ReviewID: "degraded-1", SummaryID: "degraded-summary-1", SummaryURL: "https://example.test/d-1"}, nil
}

func TestPublishStaleDegradesToSummaryOnly(t *testing.T) {
	repo := &mockWritebackItemRepo{}
	adapter := &staleThenOKAdapter{}
	orch := NewOrchestrator(OrchestratorConfig{
		Adapters: map[string]Adapter{"github": adapter},
		ItemRepo: repo,
	})

	result, err := orch.Publish(t.Context(), domain.Request{
		ReviewID:      "review-service-1",
		Provider:      "github",
		Repo:          "org/repo",
		PullRequestID: "12",
		HeadSHA:       "oldhead",
	}, domain.Result{Summary: "summary text", Findings: []domain.Finding{{
		Severity: domain.SeverityMajor,
		Path:     "a.go",
		Line:     1,
		Title:    "replace old",
		Body:     "body",
	}}}, "")
	if err != nil {
		t.Fatalf("Publish() error = %v", err)
	}
	if result == nil || !result.Stale {
		t.Fatalf("result = %+v, want Stale=true", result)
	}
	if result.ExpectedSHA != "oldhead" || result.ActualSHA != "newhead" {
		t.Fatalf("sha mismatch: %+v", result)
	}
	if adapter.calls != 2 {
		t.Fatalf("adapter calls = %d, want 2 (stale + degraded)", adapter.calls)
	}
	degraded := adapter.reviews[1]
	if degraded.ChangeRequest.HeadSHA != "newhead" {
		t.Fatalf("degraded publish anchored to %q, want newhead", degraded.ChangeRequest.HeadSHA)
	}
	if len(degraded.Comments) != 0 {
		t.Fatalf("degraded publish should have no inline comments, got %d", len(degraded.Comments))
	}
	if !strings.Contains(degraded.Summary, "本评审已过期") {
		t.Fatalf("degraded summary missing stale banner: %q", degraded.Summary)
	}
	if !strings.Contains(degraded.Summary, "summary text") {
		t.Fatalf("degraded summary missing original body: %q", degraded.Summary)
	}
	if result.Published != 1 {
		t.Fatalf("expected published=1, got %d", result.Published)
	}
	item, ok := findWritebackItem(repo.items, summaryFindingKey, "summary")
	if !ok || item.Status != "downgraded" {
		t.Fatalf("summary item = %+v ok=%v, want downgraded", item, ok)
	}
}

func TestPublishStoresServiceReviewID(t *testing.T) {
	repo := &mockWritebackItemRepo{}
	orch := NewOrchestrator(OrchestratorConfig{
		Adapters: map[string]Adapter{"github": mockWritebackAdapter{}},
		ItemRepo: repo,
	})

	diff := "diff --git a/a.go b/a.go\n--- a/a.go\n+++ b/a.go\n@@ -1,1 +1,1 @@\n-old\n+new\n"
	_, err := orch.Publish(t.Context(), domain.Request{
		ReviewID:      "review-service-1",
		Provider:      "github",
		Repo:          "org/repo",
		PullRequestID: "12",
		HeadSHA:       "abc123",
	}, domain.Result{
		Summary: "summary",
		Findings: []domain.Finding{{
			Severity: domain.SeverityMajor,
			Path:     "a.go",
			Line:     1,
			Title:    "replace old",
			Body:     "body",
		}},
	}, diff)
	if err != nil {
		t.Fatalf("Publish() error = %v", err)
	}
	if len(repo.items) != 2 {
		t.Fatalf("stored items = %d, want 2", len(repo.items))
	}
	inline, ok := findWritebackItem(repo.items, testFindingKey(), "inline")
	if !ok {
		t.Fatalf("inline item not stored: %+v", repo.items)
	}
	if inline.ReviewID != "review-service-1" {
		t.Fatalf("stored review_id = %q, want service review id", inline.ReviewID)
	}
	if inline.Attempts != 1 {
		t.Fatalf("attempts = %d, want 1", inline.Attempts)
	}
	summary, ok := findWritebackItem(repo.items, summaryFindingKey, "summary")
	if !ok || summary.ProviderExternalID != "summary-1" {
		t.Fatalf("summary item = %+v, ok=%v", summary, ok)
	}
}

func TestPublishFallsBackToProviderReviewID(t *testing.T) {
	repo := &mockWritebackItemRepo{}
	orch := NewOrchestrator(OrchestratorConfig{
		Adapters: map[string]Adapter{"github": mockWritebackAdapter{}},
		ItemRepo: repo,
	})

	_, err := orch.Publish(t.Context(), domain.Request{
		Provider:      "github",
		Repo:          "org/repo",
		PullRequestID: "12",
		HeadSHA:       "abc123",
	}, domain.Result{
		Summary: "summary",
		Findings: []domain.Finding{{
			Severity: domain.SeverityMajor,
			Path:     "a.go",
			Line:     1,
			Title:    "replace old",
			Body:     "body",
		}},
	}, "")
	if err != nil {
		t.Fatalf("Publish() error = %v", err)
	}
	if len(repo.items) != 2 {
		t.Fatalf("stored items = %d, want 2", len(repo.items))
	}
	inline, ok := findWritebackItem(repo.items, testFindingKey(), "inline")
	if !ok {
		t.Fatalf("inline item not stored: %+v", repo.items)
	}
	if inline.ReviewID != "platform-review-123" {
		t.Fatalf("stored review_id = %q, want provider fallback", inline.ReviewID)
	}
}

func TestPublishUpdatesExistingWritebackItemOnRetry(t *testing.T) {
	repo := &mockWritebackItemRepo{items: []wbitem.Item{{
		ReviewID:   "review-service-1",
		FindingKey: testFindingKey(),
		Kind:       "inline",
		Status:     "failed",
		Attempts:   1,
		LastError:  "old error",
	}}}
	orch := NewOrchestrator(OrchestratorConfig{
		Adapters: map[string]Adapter{"github": mockWritebackAdapter{}},
		ItemRepo: repo,
	})

	_, err := orch.Publish(t.Context(), domain.Request{
		ReviewID:      "review-service-1",
		Provider:      "github",
		Repo:          "org/repo",
		PullRequestID: "12",
		HeadSHA:       "abc123",
	}, domain.Result{Summary: "summary", Findings: []domain.Finding{{
		Severity: domain.SeverityMajor,
		Path:     "a.go",
		Line:     1,
		Title:    "replace old",
		Body:     "body",
	}}}, "")
	if err != nil {
		t.Fatalf("Publish() error = %v", err)
	}
	if len(repo.items) != 2 {
		t.Fatalf("stored items = %d, want 2", len(repo.items))
	}
	item, ok := findWritebackItem(repo.items, testFindingKey(), "inline")
	if !ok {
		t.Fatalf("inline item not stored: %+v", repo.items)
	}
	if item.Status != "published" || item.Attempts != 2 || item.LastError != "" {
		t.Fatalf("item after retry = %+v", item)
	}
}

func TestPublishSkipsAlreadyPublishedInlineComment(t *testing.T) {
	repo := &mockWritebackItemRepo{items: []wbitem.Item{{
		ReviewID:   "review-service-1",
		FindingKey: testFindingKey(),
		Kind:       "inline",
		Status:     "published",
		Attempts:   1,
	}}}
	adapter := &capturingWritebackAdapter{}
	orch := NewOrchestrator(OrchestratorConfig{
		Adapters: map[string]Adapter{"github": adapter},
		ItemRepo: repo,
	})

	_, err := orch.Publish(t.Context(), domain.Request{
		ReviewID:      "review-service-1",
		Provider:      "github",
		Repo:          "org/repo",
		PullRequestID: "12",
		HeadSHA:       "abc123",
	}, domain.Result{Summary: "summary", Findings: []domain.Finding{{
		Severity: domain.SeverityMajor,
		Path:     "a.go",
		Line:     1,
		Title:    "replace old",
		Body:     "body",
	}}}, "")
	if err != nil {
		t.Fatalf("Publish() error = %v", err)
	}
	if len(adapter.review.Comments) != 0 {
		t.Fatalf("published comments = %d, want 0", len(adapter.review.Comments))
	}
	item, ok := findWritebackItem(repo.items, testFindingKey(), "inline")
	if !ok || item.Attempts != 1 {
		t.Fatalf("inline item changed unexpectedly: %+v ok=%v", item, ok)
	}
}

func TestPublishSkipsWhenSummaryAndInlineAlreadyPublished(t *testing.T) {
	repo := &mockWritebackItemRepo{items: []wbitem.Item{
		{ReviewID: "review-service-1", FindingKey: summaryFindingKey, Kind: "summary", Status: "published", Attempts: 1},
		{ReviewID: "review-service-1", FindingKey: testFindingKey(), Kind: "inline", Status: "published", Attempts: 1},
	}}
	adapter := &capturingWritebackAdapter{}
	orch := NewOrchestrator(OrchestratorConfig{
		Adapters: map[string]Adapter{"github": adapter},
		ItemRepo: repo,
	})

	result, err := orch.Publish(t.Context(), domain.Request{
		ReviewID:      "review-service-1",
		Provider:      "github",
		Repo:          "org/repo",
		PullRequestID: "12",
		HeadSHA:       "abc123",
	}, domain.Result{Summary: "summary", Findings: []domain.Finding{{
		Severity: domain.SeverityMajor,
		Path:     "a.go",
		Line:     1,
		Title:    "replace old",
		Body:     "body",
	}}}, "")
	if err != nil {
		t.Fatalf("Publish() error = %v", err)
	}
	if result == nil || !result.Skipped {
		t.Fatalf("result = %+v, want skipped", result)
	}
	if adapter.review.Summary != "" || len(adapter.review.Comments) != 0 {
		t.Fatalf("adapter should not publish, got review %+v", adapter.review)
	}
}

func TestPublishWithForceRepublishesAlreadyPublishedItems(t *testing.T) {
	repo := &mockWritebackItemRepo{items: []wbitem.Item{
		{ReviewID: "review-service-1", FindingKey: summaryFindingKey, Kind: "summary", Status: "published", Attempts: 1},
		{ReviewID: "review-service-1", FindingKey: testFindingKey(), Kind: "inline", Status: "published", Attempts: 1},
	}}
	adapter := &capturingWritebackAdapter{}
	orch := NewOrchestrator(OrchestratorConfig{
		Adapters: map[string]Adapter{"github": adapter},
		ItemRepo: repo,
	})

	result, err := orch.PublishWithOptions(t.Context(), domain.Request{
		ReviewID:      "review-service-1",
		Provider:      "github",
		Repo:          "org/repo",
		PullRequestID: "12",
		HeadSHA:       "abc123",
	}, domain.Result{Summary: "summary", Findings: []domain.Finding{{
		Severity: domain.SeverityMajor,
		Path:     "a.go",
		Line:     1,
		Title:    "replace old",
		Body:     "body",
	}}}, "", PublishOptions{Force: true})
	if err != nil {
		t.Fatalf("PublishWithOptions() error = %v", err)
	}
	if result == nil || result.Skipped {
		t.Fatalf("result = %+v, want republish", result)
	}
	if adapter.review.Summary == "" || len(adapter.review.Comments) != 1 {
		t.Fatalf("adapter review = %+v, want summary and inline comment", adapter.review)
	}
	inline, ok := findWritebackItem(repo.items, testFindingKey(), "inline")
	if !ok || inline.Attempts != 2 || inline.Status != "published" {
		t.Fatalf("inline item after force publish = %+v ok=%v", inline, ok)
	}
	summary, ok := findWritebackItem(repo.items, summaryFindingKey, "summary")
	if !ok || summary.Attempts != 2 || summary.Status != "published" {
		t.Fatalf("summary item after force publish = %+v ok=%v", summary, ok)
	}
}

func TestStatusSummarizesWritebackItems(t *testing.T) {
	repo := &mockWritebackItemRepo{items: []wbitem.Item{
		{ReviewID: "r1", FindingKey: "a", Kind: "inline", Status: "published", Attempts: 1},
		{ReviewID: "r1", FindingKey: "b", Kind: "inline", Status: "failed", Attempts: 2, LastError: "bad"},
		{ReviewID: "r1", FindingKey: "c", Kind: "inline", Status: "downgraded", Attempts: 1},
		{ReviewID: "r1", FindingKey: "d", Kind: "inline", Status: "pending", Attempts: 0},
	}}
	orch := NewOrchestrator(OrchestratorConfig{ItemRepo: repo})
	status, err := orch.Status(t.Context(), "r1")
	if err != nil {
		t.Fatalf("Status() error = %v", err)
	}
	if status.Total != 4 || status.Published != 1 || status.Failed != 1 || status.Downgraded != 1 || status.Pending != 1 {
		t.Fatalf("status = %+v", status)
	}
}
