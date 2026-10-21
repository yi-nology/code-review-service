package review

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	notifdomain "github.com/yi-nology/code-review-service/internal/domain/notification"
	"github.com/yi-nology/code-review-service/internal/domain/page"
	"github.com/yi-nology/code-review-service/internal/domain/review"
	"github.com/yi-nology/code-review-service/internal/repository"
)

// reapRepo 只实现孤儿收敛/心跳/回填路径用到的 ReviewRepository 方法，
// 其余方法嵌入接口（意外调用直接 panic，暴露测试路径假设失效）。
type reapRepo struct {
	repository.ReviewRepository
	store     *fakeStateStore
	failStale []domain.Record
	listPages [][]domain.Record
	listOpts  []domain.ListOptions
	mu        sync.Mutex
	touches   int
}

func newReapRepo() *reapRepo {
	return &reapRepo{store: newFakeStateStore()}
}

func (r *reapRepo) GetByReviewID(ctx context.Context, id string) (*domain.Record, error) {
	return r.store.GetByReviewID(ctx, id)
}

func (r *reapRepo) Create(ctx context.Context, rec *domain.Record) error {
	return r.store.Create(ctx, rec)
}

func (r *reapRepo) UpdateInTransaction(ctx context.Context, id string, fn func(*domain.Record) error) error {
	return r.store.UpdateInTransaction(ctx, id, fn)
}

func (r *reapRepo) FailStaleRunning(_ context.Context, _ time.Time) ([]domain.Record, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := r.failStale
	r.failStale = nil
	return out, nil
}

func (r *reapRepo) RequeueStale(_ context.Context, _ time.Time) (int64, error) {
	return 0, nil
}

func (r *reapRepo) TouchRunning(_ context.Context, _ string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.touches++
	return nil
}

func (r *reapRepo) touchCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.touches
}

func (r *reapRepo) List(_ context.Context, opts domain.ListOptions) (*page.Page[domain.Record], error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.listOpts = append(r.listOpts, opts)
	if len(r.listPages) == 0 {
		return &page.Page[domain.Record]{Items: nil, Total: 0}, nil
	}
	items := r.listPages[0]
	r.listPages = r.listPages[1:]
	return &page.Page[domain.Record]{Items: items, Total: int64(len(items))}, nil
}

// reapPoster 捕获 commit status 写入（验证孤儿淘汰/回填补写门禁）。
type reapPoster struct {
	mu     sync.Mutex
	shas   []string
	states []string
}

func (c *reapPoster) Post(_ context.Context, _, _, sha, state, _, _ string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.shas = append(c.shas, sha)
	c.states = append(c.states, state)
}

type captureNotifier struct {
	mu     sync.Mutex
	events []notifdomain.Event
}

func (c *captureNotifier) Notify(_ context.Context, event notifdomain.Event) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.events = append(c.events, event)
	return nil
}

// captureWriteback 捕获 PR/MR 评论（失败评论走 PublishWithDiff + 仅 Summary）。
type captureWriteback struct {
	mu     sync.Mutex
	bodies []string
}

func (c *captureWriteback) PublishWithDiff(_ context.Context, _ domain.Request, result domain.Result, _ string) (*domain.WritebackResult, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.bodies = append(c.bodies, result.Summary)
	return &domain.WritebackResult{}, nil
}

func (c *captureWriteback) comments() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]string{}, c.bodies...)
}

// waitFor 轮询断言条件（失败上报的通知/评论走异步 goroutine）。
func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for {
		if cond() {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("timeout waiting for %s", what)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func newReapTestService(repo *reapRepo, poster *reapPoster, notifier *captureNotifier, commenter *captureWriteback) *Service {
	return NewService(ServiceConfig{
		ReviewRepo:        repo,
		FailureReporter:   NewFailureReporter(repo, notifier, poster).WithCommenter(commenter),
		HeartbeatInterval: 10 * time.Millisecond,
	})
}

func reapTestRequest() domain.Request {
	return domain.Request{
		Provider:        "github",
		Repo:            "acme/app",
		BaseSHA:         "a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2",
		HeadSHA:         "f6e5d4c3b2a1f6e5d4c3b2a1f6e5d4c3b2a1f6e5",
		MergeRequestIID: "7",
		GateMode:        domain.GateModeStrong,
	}
}

// TestReapOrphansReportsExhaustedReviews 核心回归：重试超限的孤儿评审被淘汰时
// 必须经 FailureReporter 补写终态门禁 status——此前直接 SQL 置 failed 不写 status，
// GitLab 侧 pending 永不收敛，MR 被永久卡死。
func TestReapOrphansReportsExhaustedReviews(t *testing.T) {
	repo := newReapRepo()
	poster := &reapPoster{}
	notifier := &captureNotifier{}
	commenter := &captureWriteback{}
	svc := newReapTestService(repo, poster, notifier, commenter)

	req := reapTestRequest()
	repo.failStale = []domain.Record{{
		ID: "exhaust-1", Provider: req.Provider, Repo: req.Repo,
		Status: statusRunning, Request: req,
	}}
	// 生产语义：被淘汰的评审本就存在于库中，预置以便历史追加/落库走更新路径。
	repo.store.records["exhaust-1"] = &domain.Record{
		ID: "exhaust-1", Provider: req.Provider, Repo: req.Repo,
		Status: statusRunning, Request: req,
	}

	svc.reapOrphans(context.Background(), time.Now().Add(-time.Hour))

	if len(poster.states) != 1 {
		t.Fatalf("expected 1 gate status post, got %d (%v)", len(poster.states), poster.states)
	}
	// 强门禁：执行失败 → error 终态（MR 释放永久 pending，作者可见并可重试）。
	if poster.states[0] != domain.StateError {
		t.Fatalf("gate status state = %q, want %q", poster.states[0], domain.StateError)
	}
	if poster.shas[0] != req.HeadSHA {
		t.Fatalf("gate status sha = %q, want %q", poster.shas[0], req.HeadSHA)
	}

	// notify/comment 走异步 goroutine，轮询避免调度竞态。
	waitFor(t, "failure notification", func() bool { return len(notifier.events) == 1 })
	waitFor(t, "failure comment", func() bool { return len(commenter.comments()) == 1 })
	if body := commenter.comments()[0]; !strings.Contains(body, "重试上限") {
		t.Fatalf("failure comment should mention retry limit, got: %s", body)
	}

	// 落库终态：response.gate 收敛为 error（回填幂等性的前提）。
	got, err := repo.GetByReviewID(context.Background(), "exhaust-1")
	if err != nil {
		t.Fatalf("GetByReviewID: %v", err)
	}
	if got.Status != statusFailed || got.Response.Gate.Status != domain.GateError {
		t.Fatalf("persisted status=%q gate=%q, want failed/%s", got.Status, got.Response.Gate.Status, domain.GateError)
	}
}

// TestFailureCommentSkippedWithoutPR 未关联 PR/MR 的评审（如仓库级 scan）失败时不评论。
func TestFailureCommentSkippedWithoutPR(t *testing.T) {
	repo := newReapRepo()
	commenter := &captureWriteback{}
	poster := &reapPoster{}
	svc := newReapTestService(repo, poster, &captureNotifier{}, commenter)

	req := reapTestRequest()
	req.MergeRequestIID = ""
	req.PullRequestID = ""
	repo.failStale = []domain.Record{{
		ID: "scan-1", Provider: req.Provider, Repo: req.Repo,
		Status: statusRunning, Request: req,
	}}
	repo.store.records["scan-1"] = &domain.Record{
		ID: "scan-1", Provider: req.Provider, Repo: req.Repo,
		Status: statusRunning, Request: req,
	}

	svc.reapOrphans(context.Background(), time.Now().Add(-time.Hour))

	if len(poster.states) != 1 {
		t.Fatalf("expected gate status still posted, got %v", poster.states)
	}
	time.Sleep(50 * time.Millisecond)
	if n := len(commenter.comments()); n != 0 {
		t.Fatalf("expected no comment for PR-less review, got %d: %v", n, commenter.comments())
	}
}

// TestReapOrphansNoExhausted 无淘汰对象时不应产生任何副作用。
func TestReapOrphansNoExhausted(t *testing.T) {
	repo := newReapRepo()
	poster := &reapPoster{}
	notifier := &captureNotifier{}
	svc := newReapTestService(repo, poster, notifier, &captureWriteback{})

	svc.reapOrphans(context.Background(), time.Now())

	if len(poster.states) != 0 || len(notifier.events) != 0 {
		t.Fatalf("unexpected side effects: posts=%v events=%v", poster.states, notifier.events)
	}
}

// TestBackfillBrokenGateFailures 回填：DB failed 但 gate 仍 pending 的历史评审
// 补写终态门禁 status；列表清空后自然终止（幂等：补写后 gate 变 error 不再命中）。
func TestBackfillBrokenGateFailures(t *testing.T) {
	repo := newReapRepo()
	poster := &reapPoster{}
	commenter := &captureWriteback{}
	svc := newReapTestService(repo, poster, &captureNotifier{}, commenter)

	req := reapTestRequest()
	repo.listPages = [][]domain.Record{{
		{
			ID: "broken-1", Provider: req.Provider, Repo: req.Repo,
			Status: statusFailed, Request: req,
			Response: domain.Response{ReviewID: "broken-1", Status: statusRunning, Gate: domain.PendingGate()},
		},
	}}

	svc.backfillBrokenGateFailures(context.Background())

	if len(poster.states) != 1 || poster.states[0] != domain.StateError {
		t.Fatalf("expected 1 error gate status post, got %v", poster.states)
	}
	waitFor(t, "backfill comment", func() bool { return len(commenter.comments()) == 1 })
	if len(repo.listOpts) != 2 {
		t.Fatalf("expected list called until empty (2 times), got %d", len(repo.listOpts))
	}
	for _, opts := range repo.listOpts {
		if opts.Status != statusFailed || opts.GateStatus != domain.GatePending {
			t.Fatalf("backfill must query failed+pending-gate only, got status=%q gate=%q", opts.Status, opts.GateStatus)
		}
		if opts.Page != 1 {
			t.Fatalf("backfill must always re-query page 1, got page %d", opts.Page)
		}
	}

	// 补写后 response.gate 收敛为 error（下次启动不再命中）。
	got, err := repo.GetByReviewID(context.Background(), "broken-1")
	if err != nil {
		t.Fatalf("GetByReviewID: %v", err)
	}
	if got.Response.Gate.Status != domain.GateError {
		t.Fatalf("gate = %q, want %q after backfill", got.Response.Gate.Status, domain.GateError)
	}
}

// TestBackfillEmptyNoOp 无受损记录时不产生副作用。
func TestBackfillEmptyNoOp(t *testing.T) {
	repo := newReapRepo()
	poster := &reapPoster{}
	svc := newReapTestService(repo, poster, &captureNotifier{}, &captureWriteback{})

	svc.backfillBrokenGateFailures(context.Background())

	if len(poster.states) != 0 {
		t.Fatalf("unexpected posts: %v", poster.states)
	}
	if len(repo.listOpts) != 1 {
		t.Fatalf("expected exactly 1 list call, got %d", len(repo.listOpts))
	}
}

// TestBackfillListErrorStops 回填查询失败时安全退出，不阻塞启动。
func TestBackfillListErrorStops(t *testing.T) {
	poster := &reapPoster{}
	failing := &failingListRepo{reapRepo: newReapRepo()}
	svc := NewService(ServiceConfig{
		ReviewRepo:        failing,
		FailureReporter:   NewFailureReporter(failing, &captureNotifier{}, poster),
		HeartbeatInterval: 10 * time.Millisecond,
	})
	svc.backfillBrokenGateFailures(context.Background())

	if len(poster.states) != 0 {
		t.Fatalf("unexpected posts on list error: %v", poster.states)
	}
}

// TestStartHeartbeat 心跳按间隔刷新 updated_at，stop 后停止。
func TestStartHeartbeat(t *testing.T) {
	repo := newReapRepo()
	svc := newReapTestService(repo, &reapPoster{}, &captureNotifier{}, &captureWriteback{})

	stop := svc.startHeartbeat("hb-1", 10*time.Millisecond)
	time.Sleep(60 * time.Millisecond)
	stop()

	n := repo.touchCount()
	if n < 2 {
		t.Fatalf("expected >=2 heartbeat touches, got %d", n)
	}
	time.Sleep(40 * time.Millisecond)
	if after := repo.touchCount(); after != n {
		t.Fatalf("heartbeat kept running after stop: %d -> %d", n, after)
	}
}

// TestStartHeartbeatDisabled 间隔 <=0 时不启动心跳。
func TestStartHeartbeatDisabled(t *testing.T) {
	repo := newReapRepo()
	svc := newReapTestService(repo, &reapPoster{}, &captureNotifier{}, &captureWriteback{})

	stop := svc.startHeartbeat("hb-1", 0)
	defer stop()
	time.Sleep(30 * time.Millisecond)
	if n := repo.touchCount(); n != 0 {
		t.Fatalf("expected no touches when disabled, got %d", n)
	}
}

// failingListRepo 覆盖 List 返回错误（回填容错路径）。
type failingListRepo struct {
	*reapRepo
}

func (f *failingListRepo) List(_ context.Context, _ domain.ListOptions) (*page.Page[domain.Record], error) {
	return nil, errors.New("db down")
}
