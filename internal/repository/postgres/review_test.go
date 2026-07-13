package postgres

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/yi-nology/code-review-service/internal/repository/postgres/models"
	"github.com/yi-nology/code-review-service/internal/repository/repoerr"
	"github.com/yi-nology/code-review-service/internal/service/review/domain"
)

// newReview 构造一条最小可入库的 models.Review。caller 填充关键字段。
func newReview(reviewID, provider, repo, status string) *domain.Record {
	return &domain.Record{
		ID:                 reviewID,
		RequestFingerprint: "fp-" + reviewID,
		Provider:           provider,
		Repo:               repo,
		Status:             status,
		PullRequestID:      "100",
		Request:            domain.Request{Provider: provider, Repo: repo, PullRequestID: "100"},
		Response:           domain.Response{ReviewID: reviewID, Status: status},
	}
}

func TestReviewRepo_CreateAndGetByReviewID(t *testing.T) {
	resetTables(t)
	ctx := context.Background()
	repo := NewReviewRepo(testDB)

	r := newReview("rev-1", "gitea", "acme/app", "queued")
	if err := repo.Create(ctx, r); err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := repo.GetByReviewID(ctx, "rev-1")
	if err != nil {
		t.Fatalf("GetByReviewID: %v", err)
	}
	if got.ID != "rev-1" || got.Provider != "gitea" || got.Status != "queued" {
		t.Fatalf("unexpected review: %+v", got)
	}
	// JSONB 列应能完整往返。注意 postgres jsonb 会规范化空白与键序，
	// 不能做字节比较，需解析后语义校验。
	if got.Request.PullRequestID != "100" {
		t.Fatalf("Request pull_request_id = %q, want 100", got.Request.PullRequestID)
	}
}

func TestReviewRepo_GetByReviewID_NotFound(t *testing.T) {
	resetTables(t)
	ctx := context.Background()
	repo := NewReviewRepo(testDB)

	_, err := repo.GetByReviewID(ctx, "missing")
	if !errors.Is(err, repoerr.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestReviewRepo_GetByIdempotencyKey(t *testing.T) {
	resetTables(t)
	ctx := context.Background()
	repo := NewReviewRepo(testDB)

	key := "idem-abc"
	r := newReview("rev-2", "github", "acme/api", "success")
	r.IdempotencyKey = key
	if err := repo.Create(ctx, r); err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := repo.GetByIdempotencyKey(ctx, key)
	if err != nil {
		t.Fatalf("GetByIdempotencyKey: %v", err)
	}
	if got.ID != "rev-2" {
		t.Fatalf("unexpected review: %s", got.ID)
	}
}

// TestReviewRepo_ActiveChangeUniqueIndex 验证 #24 TOCTOU 修复的部分唯一索引：
// 同一 (provider, repo, PR) 不能同时存在两条 active（queued/running）review。
func TestReviewRepo_ActiveChangeUniqueIndex(t *testing.T) {
	resetTables(t)
	ctx := context.Background()
	repo := NewReviewRepo(testDB)

	first := newReview("rev-a", "gitea", "acme/app", "queued")
	if err := repo.Create(ctx, first); err != nil {
		t.Fatalf("Create first: %v", err)
	}

	// 第二条同 (provider, repo, PR) 的 active review 应被索引拒绝。
	second := newReview("rev-b", "gitea", "acme/app", "queued")
	second.ID = "rev-b"
	if err := repo.Create(ctx, second); err == nil {
		t.Fatal("expected unique constraint violation for duplicate active review, got nil")
	}

	// 不同 PR 的 active review 应允许。
	third := newReview("rev-c", "gitea", "acme/app", "queued")
	third.ID = "rev-c"
	third.PullRequestID = "200"
	third.Request = domain.Request{PullRequestID: "200"}
	if err := repo.Create(ctx, third); err != nil {
		t.Fatalf("Create review with different PR should succeed: %v", err)
	}

	// 已结束（success）的 review 不受索引约束，可与同 PR 的 active 共存。
	if err := repo.UpdateInTransaction(ctx, "rev-a", func(r *domain.Record) error {
		r.Status = "success"
		return nil
	}); err != nil {
		t.Fatalf("mark rev-a success: %v", err)
	}
	fourth := newReview("rev-d", "gitea", "acme/app", "queued")
	fourth.ID = "rev-d"
	if err := repo.Create(ctx, fourth); err != nil {
		t.Fatalf("Create active review after prior one completed should succeed: %v", err)
	}
}

func TestReviewRepo_GetActiveByProviderRepo(t *testing.T) {
	resetTables(t)
	ctx := context.Background()
	repo := NewReviewRepo(testDB)

	// 一条 success（不应被 active 查询命中）+ 一条 queued（应命中）。
	done := newReview("rev-done", "github", "acme/api", "success")
	done.PullRequestID = "42"
	done.Request = domain.Request{PullRequestID: "42"}
	if err := repo.Create(ctx, done); err != nil {
		t.Fatalf("Create done: %v", err)
	}
	active := newReview("rev-active", "github", "acme/api", "running")
	active.PullRequestID = "42"
	active.Request = domain.Request{PullRequestID: "42"}
	if err := repo.Create(ctx, active); err != nil {
		t.Fatalf("Create active: %v", err)
	}

	got, err := repo.GetActiveByProviderRepo(ctx, "github", "acme/api", "42", "")
	if err != nil {
		t.Fatalf("GetActiveByProviderRepo: %v", err)
	}
	if got.ID != "rev-active" {
		t.Fatalf("expected rev-active, got %s", got.ID)
	}

	// 不存在的 PR。
	if _, err := repo.GetActiveByProviderRepo(ctx, "github", "acme/api", "999", ""); !errors.Is(err, repoerr.ErrNotFound) {
		t.Fatalf("expected ErrNotFound for unknown PR, got %v", err)
	}
}

func TestReviewRepo_ClaimQueued(t *testing.T) {
	resetTables(t)
	ctx := context.Background()
	repo := NewReviewRepo(testDB)

	// 两条 queued（PR 不同，避免唯一索引冲突）。
	if err := repo.Create(ctx, newReview("q-1", "gitea", "acme/app", "queued")); err != nil {
		t.Fatalf("Create q-1: %v", err)
	}
	two := newReview("q-2", "gitea", "acme/app", "queued")
	two.PullRequestID = "200"
	two.Request = domain.Request{PullRequestID: "200"}
	if err := repo.Create(ctx, two); err != nil {
		t.Fatalf("Create q-2: %v", err)
	}
	// 一条 running（不应被 claim）。
	running := newReview("r-1", "gitea", "acme/app", "running")
	running.PullRequestID = "300"
	running.Request = domain.Request{PullRequestID: "300"}
	if err := repo.Create(ctx, running); err != nil {
		t.Fatalf("Create r-1: %v", err)
	}

	claimed, err := repo.ClaimQueued(ctx, 10)
	if err != nil {
		t.Fatalf("ClaimQueued: %v", err)
	}
	if len(claimed) != 2 {
		t.Fatalf("expected 2 claimed, got %d", len(claimed))
	}
	// claim 后状态应变 running。
	for _, c := range claimed {
		if c.Status != "running" {
			t.Fatalf("claimed review %s status = %q, want running", c.ID, c.Status)
		}
	}
	// 再次 claim 应返回 0（已无 queued）。
	again, err := repo.ClaimQueued(ctx, 10)
	if err != nil {
		t.Fatalf("ClaimQueued again: %v", err)
	}
	if len(again) != 0 {
		t.Fatalf("expected 0 claimed on second pass, got %d", len(again))
	}
}

func TestReviewRepo_RequeueStale(t *testing.T) {
	resetTables(t)
	ctx := context.Background()
	repo := NewReviewRepo(testDB)

	stale := newReview("stale-1", "gitea", "acme/app", "running")
	stale.UpdatedAt = time.Now().Add(-2 * time.Hour)
	if err := repo.Create(ctx, stale); err != nil {
		t.Fatalf("Create stale: %v", err)
	}

	fresh := newReview("fresh-1", "gitea", "acme/app", "running")
	fresh.PullRequestID = "200"
	fresh.Request = domain.Request{PullRequestID: "200"}
	fresh.UpdatedAt = time.Now()
	if err := repo.Create(ctx, fresh); err != nil {
		t.Fatalf("Create fresh: %v", err)
	}

	// 重排 1 小时前更新的 running 评审：stale 被重排，fresh 不动。
	n, err := repo.RequeueStale(ctx, time.Now().Add(-1*time.Hour))
	if err != nil {
		t.Fatalf("RequeueStale: %v", err)
	}
	if n != 1 {
		t.Fatalf("expected 1 requeued, got %d", n)
	}

	got, _ := repo.GetByReviewID(ctx, "stale-1")
	if got.Status != "queued" {
		t.Fatalf("stale review status = %q, want queued", got.Status)
	}
	freshGot, _ := repo.GetByReviewID(ctx, "fresh-1")
	if freshGot.Status != "running" {
		t.Fatalf("fresh review status = %q, want running", freshGot.Status)
	}
}

// TestReviewRepo_RequeueStale_RetryLimit 验证 RequeueStale 的雪崩防护：
// retry_count 未达上限的重新入队且计数 +1，达到上限的标记 failed 不再重排。
func TestReviewRepo_RequeueStale_RetryLimit(t *testing.T) {
	resetTables(t)
	ctx := context.Background()
	repo := NewReviewRepo(testDB)

	// 可以重排的孤儿（retry_count 在上限内）
	retryable := newReview("retry-1", "gitea", "acme/app", "running")
	if err := repo.Create(ctx, retryable); err != nil {
		t.Fatalf("Create retryable: %v", err)
	}
	if err := testDB.Model(&models.Review{}).
		Where("review_id = ?", "retry-1").
		Update("updated_at", time.Now().Add(-2*time.Hour)).Error; err != nil {
		t.Fatalf("set updated_at: %v", err)
	}

	// 已达重试上限的孤儿（应标记 failed 不再重排）
	exhausted := newReview("exhaust-1", "gitea", "acme/other", "running")
	if err := repo.Create(ctx, exhausted); err != nil {
		t.Fatalf("Create exhausted: %v", err)
	}
	// 直接通过 DB 设置 retry_count 为上限 + updated_at 为 2 小时前（绕过 GORM 自动时间管理）。
	if err := testDB.Model(&models.Review{}).
		Where("review_id = ?", "exhaust-1").
		Updates(map[string]interface{}{
			"retry_count": MaxRequeueRetries,
			"updated_at":  time.Now().Add(-2 * time.Hour),
		}).Error; err != nil {
		t.Fatalf("set retry_count+updated_at: %v", err)
	}

	n, err := repo.RequeueStale(ctx, time.Now().Add(-1*time.Hour))
	if err != nil {
		t.Fatalf("RequeueStale: %v", err)
	}
	if n != 2 {
		t.Fatalf("expected 2 affected (1 requeued + 1 failed), got %d", n)
	}

	// retryable → queued，retry_count 应 +1
	gotRetry, _ := repo.GetByReviewID(ctx, "retry-1")
	if gotRetry.Status != "queued" {
		t.Fatalf("retryable status = %q, want queued", gotRetry.Status)
	}
	var rc1 int
	testDB.Model(&models.Review{}).Where("review_id = ?", "retry-1").Pluck("retry_count", &rc1)
	if rc1 != 1 {
		t.Fatalf("retryable retry_count = %d, want 1", rc1)
	}

	// exhausted → failed，不再重排
	gotExhausted, _ := repo.GetByReviewID(ctx, "exhaust-1")
	if gotExhausted.Status != "failed" {
		t.Fatalf("exhausted status = %q, want failed (retry limit reached)", gotExhausted.Status)
	}
}

func TestReviewRepo_UpdateInTransaction(t *testing.T) {
	resetTables(t)
	ctx := context.Background()
	repo := NewReviewRepo(testDB)

	if err := repo.Create(ctx, newReview("tx-1", "gitea", "acme/app", "queued")); err != nil {
		t.Fatalf("Create: %v", err)
	}

	// 回调内改写 Response JSONB + Status，验证事务落库。
	if err := repo.UpdateInTransaction(ctx, "tx-1", func(r *domain.Record) error {
		r.Status = "success"
		r.Response = domain.Response{Status: "success"}
		r.Score = 85
		return nil
	}); err != nil {
		t.Fatalf("UpdateInTransaction: %v", err)
	}

	got, _ := repo.GetByReviewID(ctx, "tx-1")
	if got.Status != "success" || got.Score != 85 {
		t.Fatalf("tx update not persisted: status=%q score=%d", got.Status, got.Score)
	}
	if got.Response.Status != "success" {
		t.Fatalf("Response status = %q, want success", got.Response.Status)
	}

	// 不存在的 review 应返回 ErrNotFound。
	if err := repo.UpdateInTransaction(ctx, "missing", func(r *domain.Record) error { return nil }); !errors.Is(err, repoerr.ErrNotFound) {
		t.Fatalf("expected ErrNotFound for missing review, got %v", err)
	}
}

func TestReviewRepo_Save(t *testing.T) {
	resetTables(t)
	ctx := context.Background()
	repo := NewReviewRepo(testDB)

	r := newReview("save-1", "gitea", "acme/app", "queued")
	if err := repo.Create(ctx, r); err != nil {
		t.Fatalf("Create: %v", err)
	}
	r.Status = "success"
	r.Grade = "A"
	if err := repo.Save(ctx, r); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, _ := repo.GetByReviewID(ctx, "save-1")
	if got.Status != "success" || got.Grade != "A" {
		t.Fatalf("Save not persisted: status=%q grade=%q", got.Status, got.Grade)
	}
}
