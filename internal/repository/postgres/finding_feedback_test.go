package postgres

import (
	"context"
	"testing"

	"github.com/yi-nology/code-review-service/internal/domain/feedback"
)

func TestFindingFeedbackRepo_UpsertAndList(t *testing.T) {
	resetTables(t)
	ctx := context.Background()
	repo := NewFindingFeedbackRepo(testDB)
	// finding_feedback 表需要单独清空（resetTables 默认只清 reviews/review_sessions）。
	if err := truncateTables(t, "finding_feedback"); err != nil {
		t.Fatalf("truncate: %v", err)
	}

	// 新增两条反馈。
	f1 := &feedback.Feedback{ReviewID: "rev-1", FindingIndex: 0, Status: "false_positive", Note: "not applicable"}
	f2 := &feedback.Feedback{ReviewID: "rev-1", FindingIndex: 1, Status: "accepted_risk", Note: "low impact"}
	if err := repo.Upsert(ctx, f1); err != nil {
		t.Fatalf("Upsert f1: %v", err)
	}
	if err := repo.Upsert(ctx, f2); err != nil {
		t.Fatalf("Upsert f2: %v", err)
	}

	items, err := repo.ListByReview(ctx, "rev-1")
	if err != nil {
		t.Fatalf("ListByReview: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
	// 应按 finding_index ASC 排序。
	if items[0].FindingIndex != 0 || items[1].FindingIndex != 1 {
		t.Fatalf("unexpected order: %+v", items)
	}
	if items[0].Status != "false_positive" || items[1].Status != "accepted_risk" {
		t.Fatalf("unexpected statuses: %+v", items)
	}

	// Upsert 同 (review_id, finding_index) 应更新而非插入（OnConflict）。
	f1Update := &feedback.Feedback{ReviewID: "rev-1", FindingIndex: 0, Status: "fixed", Note: "patched"}
	if err := repo.Upsert(ctx, f1Update); err != nil {
		t.Fatalf("Upsert update: %v", err)
	}
	items, _ = repo.ListByReview(ctx, "rev-1")
	if len(items) != 2 {
		t.Fatalf("expected still 2 items after upsert, got %d", len(items))
	}
	if items[0].Status != "fixed" {
		t.Fatalf("status not updated by upsert: %s", items[0].Status)
	}

	// ListByReview 不同 review 应返回空。
	other, _ := repo.ListByReview(ctx, "rev-other")
	if len(other) != 0 {
		t.Fatalf("expected 0 items for other review, got %d", len(other))
	}
}
