package postgres

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/yi-nology/code-review-service/internal/domain/review"
	"github.com/yi-nology/code-review-service/internal/repository/postgres/models"
	"github.com/yi-nology/code-review-service/internal/repository/repoerr"
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

// TestReviewRepo_RequeueStale_RetryLimit 验证 stale 收敛的分工：
// RequeueStale 只重排未达上限的（计数 +1）；达上限的保持 running，
// 由 FailStaleRunning 淘汰并返回记录供上层补写门禁 status。
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

	// 已达重试上限的孤儿（RequeueStale 不动，交给 FailStaleRunning）
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
	if n != 1 {
		t.Fatalf("expected 1 requeued, got %d", n)
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

	// exhausted 保持 running（淘汰是 FailStaleRunning 的职责）
	gotExhausted, _ := repo.GetByReviewID(ctx, "exhaust-1")
	if gotExhausted.Status != "running" {
		t.Fatalf("exhausted status = %q, want running (RequeueStale must not touch it)", gotExhausted.Status)
	}
}

// TestReviewRepo_FailStaleRunning 验证淘汰路径：stale 且重试达上限的 running 评审
// 被置 failed 并连同 Request 一起返回（供 FailureReporter 补写门禁 status）；
// 未达上限或 fresh 的评审不受影响。
func TestReviewRepo_FailStaleRunning(t *testing.T) {
	resetTables(t)
	ctx := context.Background()
	repo := NewReviewRepo(testDB)

	staleExhausted := newReview("exhaust-1", "gitea", "acme/app", "running")
	staleExhausted.Request = domain.Request{Provider: "gitea", Repo: "acme/app", HeadSHA: "abc123"}
	if err := repo.Create(ctx, staleExhausted); err != nil {
		t.Fatalf("Create: %v", err)
	}
	// 同一 (provider, repo, change) 的 active 唯一索引约束：错开 change。
	freshExhausted := newReview("exhaust-fresh", "gitea", "acme/app", "running")
	freshExhausted.PullRequestID = "201"
	if err := repo.Create(ctx, freshExhausted); err != nil {
		t.Fatalf("Create: %v", err)
	}
	retryable := newReview("retry-1", "gitea", "acme/app", "running")
	retryable.PullRequestID = "202"
	if err := repo.Create(ctx, retryable); err != nil {
		t.Fatalf("Create: %v", err)
	}
	// exhaust-1: stale + 达上限；exhaust-fresh: fresh + 达上限；retry-1: stale + 未达上限
	// exhaust-1: stale + 达上限（一次性 Updates，避免 GORM autoUpdateTime 刷掉 stale 时间）；
	// exhaust-fresh: fresh + 达上限；retry-1: stale + 未达上限。
	if err := testDB.Model(&models.Review{}).
		Where("review_id = ?", "exhaust-1").
		Updates(map[string]interface{}{
			"retry_count": MaxRequeueRetries,
			"updated_at":  time.Now().Add(-2 * time.Hour),
		}).Error; err != nil {
		t.Fatalf("set exhaust-1: %v", err)
	}
	if err := testDB.Model(&models.Review{}).
		Where("review_id = ?", "exhaust-fresh").
		Update("retry_count", MaxRequeueRetries).Error; err != nil {
		t.Fatalf("set exhaust-fresh: %v", err)
	}
	if err := testDB.Model(&models.Review{}).
		Where("review_id = ?", "retry-1").
		Update("updated_at", time.Now().Add(-2*time.Hour)).Error; err != nil {
		t.Fatalf("set retry-1: %v", err)
	}

	failed, err := repo.FailStaleRunning(ctx, time.Now().Add(-1*time.Hour))
	if err != nil {
		t.Fatalf("FailStaleRunning: %v", err)
	}
	if len(failed) != 1 || failed[0].ID != "exhaust-1" {
		t.Fatalf("expected exactly [exhaust-1], got %+v", failed)
	}
	if failed[0].Request.HeadSHA != "abc123" {
		t.Fatalf("returned record must carry original Request, got %+v", failed[0].Request)
	}

	got, _ := repo.GetByReviewID(ctx, "exhaust-1")
	if got.Status != "failed" {
		t.Fatalf("exhaust-1 status = %q, want failed", got.Status)
	}
	for _, id := range []string{"exhaust-fresh", "retry-1"} {
		got, _ = repo.GetByReviewID(ctx, id)
		if got.Status != "running" {
			t.Fatalf("%s status = %q, want running", id, got.Status)
		}
	}

	// 再跑一次：已置 failed 的不再命中（幂等）。
	again, err := repo.FailStaleRunning(ctx, time.Now().Add(-1*time.Hour))
	if err != nil {
		t.Fatalf("FailStaleRunning again: %v", err)
	}
	if len(again) != 0 {
		t.Fatalf("expected 0 on second run, got %+v", again)
	}
}

// TestReviewRepo_TouchRunning 验证心跳只刷新 running 评审的 updated_at，
// 终态评审不受影响。
func TestReviewRepo_TouchRunning(t *testing.T) {
	resetTables(t)
	ctx := context.Background()
	repo := NewReviewRepo(testDB)

	running := newReview("hb-running", "gitea", "acme/app", "running")
	if err := repo.Create(ctx, running); err != nil {
		t.Fatalf("Create: %v", err)
	}
	done := newReview("hb-done", "gitea", "acme/app", "success")
	if err := repo.Create(ctx, done); err != nil {
		t.Fatalf("Create: %v", err)
	}
	old := time.Now().Add(-2 * time.Hour)
	if err := testDB.Model(&models.Review{}).
		Where("review_id IN ?", []string{"hb-running", "hb-done"}).
		Update("updated_at", old).Error; err != nil {
		t.Fatalf("set updated_at: %v", err)
	}

	if err := repo.TouchRunning(ctx, "hb-running"); err != nil {
		t.Fatalf("TouchRunning: %v", err)
	}
	if err := repo.TouchRunning(ctx, "hb-done"); err != nil {
		t.Fatalf("TouchRunning (terminal): %v", err)
	}

	var gotRunning models.Review
	if err := testDB.Where("review_id = ?", "hb-running").First(&gotRunning).Error; err != nil {
		t.Fatalf("get hb-running: %v", err)
	}
	if gotRunning.UpdatedAt.Before(time.Now().Add(-time.Minute)) {
		t.Fatalf("hb-running updated_at not refreshed: %v", gotRunning.UpdatedAt)
	}
	// 注意用全新变量：复用带主键的 struct 会让 GORM 附加 id=1 条件。
	var gotDone models.Review
	if err := testDB.Where("review_id = ?", "hb-done").First(&gotDone).Error; err != nil {
		t.Fatalf("get hb-done: %v", err)
	}
	if !gotDone.UpdatedAt.Before(time.Now().Add(-time.Minute)) {
		t.Fatalf("hb-done updated_at must stay stale: %v", gotDone.UpdatedAt)
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

// TestReviewRepo_ListEngineFilter 验证按生效审查引擎过滤：engine 存于 Request JSONB
// （生效值，见 create.go resolveEngine），历史记录无该键时一律视为 ocr；
// 列表与聚合统计使用同一过滤语义。
func TestReviewRepo_ListEngineFilter(t *testing.T) {
	resetTables(t)
	ctx := context.Background()
	repo := NewReviewRepo(testDB)

	seed := func(id, engine, status string) {
		r := newReview(id, "gitea", "acme/app", status)
		r.Request.Engine = engine
		if err := repo.Create(ctx, r); err != nil {
			t.Fatalf("Create %s: %v", id, err)
		}
	}
	seed("rev-engine-ocr", domain.EngineOCR, "success")
	seed("rev-engine-argus", domain.EngineArgus, "running")
	// 历史 OCR 记录：Request.Engine 为空（json omitempty），JSONB 里没有 engine 键。
	seed("rev-engine-legacy", "", "success")

	listIDs := func(engine string) map[string]bool {
		got, err := repo.List(ctx, domain.ListOptions{Engine: engine, Page: 1, PageSize: 50})
		if err != nil {
			t.Fatalf("List(engine=%q): %v", engine, err)
		}
		ids := make(map[string]bool, len(got.Items))
		for _, item := range got.Items {
			ids[item.ID] = true
		}
		return ids
	}

	ocrIDs := listIDs(domain.EngineOCR)
	if len(ocrIDs) != 2 || !ocrIDs["rev-engine-ocr"] || !ocrIDs["rev-engine-legacy"] {
		t.Fatalf("engine=ocr 应含显式 ocr 与历史记录两条，got %v", ocrIDs)
	}
	argusIDs := listIDs(domain.EngineArgus)
	if len(argusIDs) != 1 || !argusIDs["rev-engine-argus"] {
		t.Fatalf("engine=argus 应只有 argus 一条，got %v", argusIDs)
	}
	if all := listIDs(""); len(all) != 3 {
		t.Fatalf("engine 留空应不过滤（3 条），got %v", all)
	}

	// 聚合统计同语义：running 只统计当前引擎的记录。
	agg, err := repo.AggregateStats(ctx, domain.ListOptions{Engine: domain.EngineArgus})
	if err != nil {
		t.Fatalf("AggregateStats(engine=argus): %v", err)
	}
	if agg.Running != 1 {
		t.Fatalf("engine=argus running = %d, want 1", agg.Running)
	}
	aggOCR, err := repo.AggregateStats(ctx, domain.ListOptions{Engine: domain.EngineOCR})
	if err != nil {
		t.Fatalf("AggregateStats(engine=ocr): %v", err)
	}
	if aggOCR.Running != 0 {
		t.Fatalf("engine=ocr running = %d, want 0", aggOCR.Running)
	}
}
