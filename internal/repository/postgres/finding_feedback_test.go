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

// TestFindingFeedbackRepo_FingerprintAndLookup 验证指纹持久化与按指纹跨 review 查询，
// 这是误报抑制（同一指纹曾标记 false_positive → 后续降级）的数据基础。
func TestFindingFeedbackRepo_FingerprintAndLookup(t *testing.T) {
	resetTables(t)
	ctx := context.Background()
	repo := NewFindingFeedbackRepo(testDB)
	if err := truncateTables(t, "finding_feedback"); err != nil {
		t.Fatalf("truncate: %v", err)
	}

	// rev-1 的 finding-0 标记误报，携带指纹 fp-aaa。
	f := &feedback.Feedback{
		ReviewID: "rev-1", FindingIndex: 0, Fingerprint: "fp-aaa",
		Status: "false_positive", Note: "not real",
	}
	if err := repo.Upsert(ctx, f); err != nil {
		t.Fatalf("Upsert: %v", err)
	}

	// 验证指纹已持久化。
	items, _ := repo.ListByReview(ctx, "rev-1")
	if len(items) != 1 || items[0].Fingerprint != "fp-aaa" {
		t.Fatalf("fingerprint not persisted: %+v", items)
	}

	// rev-2 的同一指纹标记 fixed（模拟跨 review 反馈）。
	f2 := &feedback.Feedback{
		ReviewID: "rev-2", FindingIndex: 3, Fingerprint: "fp-aaa",
		Status: "fixed",
	}
	_ = repo.Upsert(ctx, f2)
	// 另一个无关指纹。
	f3 := &feedback.Feedback{
		ReviewID: "rev-2", FindingIndex: 4, Fingerprint: "fp-bbb",
		Status: "accepted_risk",
	}
	_ = repo.Upsert(ctx, f3)

	// 按指纹查询：fp-aaa 应返回 2 条（跨 rev-1/rev-2）。
	got, err := repo.ListByFingerprints(ctx, []string{"fp-aaa"})
	if err != nil {
		t.Fatalf("ListByFingerprints: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 feedback for fp-aaa (cross-review), got %d", len(got))
	}

	// 查不存在的指纹应返回空。
	none, _ := repo.ListByFingerprints(ctx, []string{"fp-zzz"})
	if len(none) != 0 {
		t.Fatalf("expected 0 for unknown fingerprint, got %d", len(none))
	}

	// 空入参应直接返回 nil，不查库。
	empty, _ := repo.ListByFingerprints(ctx, nil)
	if empty != nil {
		t.Fatalf("expected nil for empty input, got %v", empty)
	}
}
