package writeback

import (
	"context"
	"testing"

	"github.com/yi-nology/code-review-service/internal/repository/postgres/models"
	"github.com/yi-nology/code-review-service/internal/service/review"
)

type mockWritebackItemRepo struct {
	items []models.WritebackItem
}

func (m *mockWritebackItemRepo) Create(_ context.Context, item *models.WritebackItem) error {
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

func (m *mockWritebackItemRepo) ListByReview(context.Context, string) ([]models.WritebackItem, error) {
	return m.items, nil
}

func (m *mockWritebackItemRepo) ListRetryable(context.Context, int) ([]models.WritebackItem, error) {
	return m.items, nil
}

type mockWritebackAdapter struct {
	result Result
}

func (mockWritebackAdapter) Capabilities(context.Context, ChangeRequest) (Capabilities, error) {
	return Capabilities{BatchReview: true, MultiLineComments: true, Suggestions: true}, nil
}

func (m mockWritebackAdapter) Publish(context.Context, Review) (Result, error) {
	if m.result.ReviewID != "" || len(m.result.Comments) > 0 {
		return m.result, nil
	}
	return Result{
		ReviewID: "platform-review-123",
		Comments: []CommentResult{{
			Key:        "finding-key",
			ExternalID: "comment-1",
			URL:        "https://example.test/comment-1",
			Inline:     true,
		}},
	}, nil
}

func TestPublishStoresServiceReviewID(t *testing.T) {
	repo := &mockWritebackItemRepo{}
	orch := NewOrchestrator(OrchestratorConfig{
		Adapters: map[string]Adapter{"github": mockWritebackAdapter{}},
		ItemRepo: repo,
	})

	diff := "diff --git a/a.go b/a.go\n--- a/a.go\n+++ b/a.go\n@@ -1,1 +1,1 @@\n-old\n+new\n"
	_, err := orch.Publish(t.Context(), review.Request{
		ReviewID:      "review-service-1",
		Provider:      "github",
		Repo:          "org/repo",
		PullRequestID: "12",
		HeadSHA:       "abc123",
	}, review.Result{
		Summary: "summary",
		Findings: []review.Finding{{
			Severity: review.SeverityMajor,
			Path:     "a.go",
			Line:     1,
			Title:    "replace old",
			Body:     "body",
		}},
	}, diff)
	if err != nil {
		t.Fatalf("Publish() error = %v", err)
	}
	if len(repo.items) != 1 {
		t.Fatalf("stored items = %d, want 1", len(repo.items))
	}
	if repo.items[0].ReviewID != "review-service-1" {
		t.Fatalf("stored review_id = %q, want service review id", repo.items[0].ReviewID)
	}
	if repo.items[0].Attempts != 1 {
		t.Fatalf("attempts = %d, want 1", repo.items[0].Attempts)
	}
}

func TestPublishFallsBackToProviderReviewID(t *testing.T) {
	repo := &mockWritebackItemRepo{}
	orch := NewOrchestrator(OrchestratorConfig{
		Adapters: map[string]Adapter{"github": mockWritebackAdapter{}},
		ItemRepo: repo,
	})

	_, err := orch.Publish(t.Context(), review.Request{
		Provider:      "github",
		Repo:          "org/repo",
		PullRequestID: "12",
		HeadSHA:       "abc123",
	}, review.Result{
		Summary: "summary",
		Findings: []review.Finding{{
			Severity: review.SeverityMajor,
			Path:     "a.go",
			Line:     1,
			Title:    "replace old",
			Body:     "body",
		}},
	}, "")
	if err != nil {
		t.Fatalf("Publish() error = %v", err)
	}
	if len(repo.items) != 1 {
		t.Fatalf("stored items = %d, want 1", len(repo.items))
	}
	if repo.items[0].ReviewID != "platform-review-123" {
		t.Fatalf("stored review_id = %q, want provider fallback", repo.items[0].ReviewID)
	}
}

func TestPublishUpdatesExistingWritebackItemOnRetry(t *testing.T) {
	repo := &mockWritebackItemRepo{items: []models.WritebackItem{{
		ReviewID:   "review-service-1",
		FindingKey: "finding-key",
		Kind:       "inline",
		Status:     "failed",
		Attempts:   1,
		LastError:  "old error",
	}}}
	orch := NewOrchestrator(OrchestratorConfig{
		Adapters: map[string]Adapter{"github": mockWritebackAdapter{}},
		ItemRepo: repo,
	})

	_, err := orch.Publish(t.Context(), review.Request{
		ReviewID:      "review-service-1",
		Provider:      "github",
		Repo:          "org/repo",
		PullRequestID: "12",
		HeadSHA:       "abc123",
	}, review.Result{Summary: "summary", Findings: []review.Finding{{
		Severity: review.SeverityMajor,
		Path:     "a.go",
		Line:     1,
		Title:    "replace old",
		Body:     "body",
	}}}, "")
	if err != nil {
		t.Fatalf("Publish() error = %v", err)
	}
	if len(repo.items) != 1 {
		t.Fatalf("stored items = %d, want 1", len(repo.items))
	}
	item := repo.items[0]
	if item.Status != "published" || item.Attempts != 2 || item.LastError != "" {
		t.Fatalf("item after retry = %+v", item)
	}
}

func TestStatusSummarizesWritebackItems(t *testing.T) {
	repo := &mockWritebackItemRepo{items: []models.WritebackItem{
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
