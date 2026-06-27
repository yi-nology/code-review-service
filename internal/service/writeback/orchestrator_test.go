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
	m.items = append(m.items, *item)
	return nil
}

func (m *mockWritebackItemRepo) UpdateStatus(context.Context, string, string, string, string, string, string, string) error {
	return nil
}

func (m *mockWritebackItemRepo) IncrementAttempts(context.Context, string, string, string) error {
	return nil
}

func (m *mockWritebackItemRepo) ListByReview(context.Context, string) ([]models.WritebackItem, error) {
	return m.items, nil
}

func (m *mockWritebackItemRepo) ListRetryable(context.Context, int) ([]models.WritebackItem, error) {
	return m.items, nil
}

type mockWritebackAdapter struct{}

func (mockWritebackAdapter) Capabilities(context.Context, ChangeRequest) (Capabilities, error) {
	return Capabilities{BatchReview: true, MultiLineComments: true, Suggestions: true}, nil
}

func (mockWritebackAdapter) Publish(context.Context, Review) (Result, error) {
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
