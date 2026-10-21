package review

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/yi-nology/code-review-service/internal/domain/page"

	"github.com/yi-nology/code-review-service/internal/argusclient"
	"github.com/yi-nology/code-review-service/internal/domain/argusingest"
	"github.com/yi-nology/code-review-service/internal/domain/projectpolicy"
	"github.com/yi-nology/code-review-service/internal/domain/review"
	"github.com/yi-nology/code-review-service/internal/repository"
	"github.com/yi-nology/code-review-service/internal/repository/repoerr"
	"github.com/yi-nology/code-review-service/internal/service/progress"
)

// fakePolicyRepo 按需覆盖 GetByProviderRepo；其余方法嵌入接口零值（不会被走到）。
type fakePolicyRepo struct {
	repository.ProjectPolicyRepository
	policy *projectpolicy.ProjectPolicy
}

func (f *fakePolicyRepo) GetByProviderRepo(_ context.Context, _, _ string) (*projectpolicy.ProjectPolicy, error) {
	return f.policy, nil
}

// fakeReviewRepoForArgus 只覆盖 argus 执行器/轮询器用到的四个方法。
type fakeReviewRepoForArgus struct {
	repository.ReviewRepository
	saved map[string]*domain.Record
}

func (f *fakeReviewRepoForArgus) UpdateInTransaction(_ context.Context, reviewID string, fn func(*domain.Record) error) error {
	rec, ok := f.saved[reviewID]
	if !ok {
		rec = &domain.Record{ID: reviewID, Response: domain.Response{ReviewID: reviewID}}
		f.saved[reviewID] = rec
	}
	return fn(rec)
}

func (f *fakeReviewRepoForArgus) List(_ context.Context, opts domain.ListOptions) (*page.Page[domain.Record], error) {
	items := make([]domain.Record, 0, len(f.saved))
	for _, rec := range f.saved {
		items = append(items, *rec)
	}
	return &page.Page[domain.Record]{Items: items}, nil
}

func (f *fakeReviewRepoForArgus) GetByReviewID(_ context.Context, reviewID string) (*domain.Record, error) {
	if rec, ok := f.saved[reviewID]; ok {
		return rec, nil
	}
	return nil, repoerr.ErrNotFound
}

// GetActiveByProviderRepo 返回第一条 running 记录（轮询器恢复路径的匹配语义在
// 单测场景下足够：同仓同时只有一条 active）。
func (f *fakeReviewRepoForArgus) GetActiveByProviderRepo(_ context.Context, _, _, _, _ string) (*domain.Record, error) {
	for _, rec := range f.saved {
		if rec.Status == statusRunning {
			return rec, nil
		}
	}
	return nil, repoerr.ErrNotFound
}

// fakeArgusReviewRepo 承接表 fake：记录 upsert 的上报实体。
type fakeArgusReviewRepo struct {
	repository.ArgusReviewRepository
	upserts []*argusingest.Review
	fail    bool
}

func (f *fakeArgusReviewRepo) UpsertReview(_ context.Context, rev *argusingest.Review) (uint, error) {
	if f.fail {
		return 0, fmt.Errorf("upsert fail")
	}
	f.upserts = append(f.upserts, rev)
	return 7, nil
}

func TestResolveEngine(t *testing.T) {
	ctx := context.Background()
	mk := func(policyEngine string, argusEnabled bool) *Service {
		return NewService(ServiceConfig{
			ProjectRepo:  &fakePolicyRepo{policy: &projectpolicy.ProjectPolicy{ReviewEngine: policyEngine}},
			ArgusEnabled: argusEnabled,
		})
	}

	t.Run("默认 OCR", func(t *testing.T) {
		s := mk("", true)
		req := domain.Request{}
		if err := s.resolveEngine(ctx, &req); err != nil || req.Engine != domain.EngineOCR {
			t.Fatalf("engine=%q err=%v", req.Engine, err)
		}
	})
	t.Run("请求显式优先于项目策略", func(t *testing.T) {
		s := mk("argus", true)
		req := domain.Request{Engine: domain.EngineOCR}
		if err := s.resolveEngine(ctx, &req); err != nil || req.Engine != domain.EngineOCR {
			t.Fatalf("engine=%q err=%v", req.Engine, err)
		}
	})
	t.Run("项目策略 argus 生效", func(t *testing.T) {
		s := mk("Argus", true)
		req := domain.Request{Provider: "gitea"}
		if err := s.resolveEngine(ctx, &req); err != nil || req.Engine != domain.EngineArgus {
			t.Fatalf("engine=%q err=%v", req.Engine, err)
		}
	})
	t.Run("argus 未配置 fail-fast", func(t *testing.T) {
		s := mk("argus", false)
		req := domain.Request{}
		if err := s.resolveEngine(ctx, &req); err == nil {
			t.Fatal("未配置 argus 应拒绝")
		}
	})
	t.Run("请求显式 argus 未配置同样拒绝", func(t *testing.T) {
		s := mk("", false)
		req := domain.Request{Engine: domain.EngineArgus}
		if err := s.resolveEngine(ctx, &req); err == nil {
			t.Fatal("应拒绝")
		}
	})
	t.Run("argus 平台不支持 fail-fast（gerrit）", func(t *testing.T) {
		s := mk("argus", true)
		req := domain.Request{Provider: "gerrit", Repo: "proj/demo"}
		if err := s.resolveEngine(ctx, &req); err == nil {
			t.Fatal("gerrit + argus 策略应创建期拒绝")
		}
	})
	t.Run("argus 支持平台放行", func(t *testing.T) {
		s := mk("argus", true)
		req := domain.Request{Provider: "gitea", Repo: "o/demo"}
		if err := s.resolveEngine(ctx, &req); err != nil || req.Engine != domain.EngineArgus {
			t.Fatalf("err=%v engine=%q", err, req.Engine)
		}
	})
}

func TestValidateEngine(t *testing.T) {
	t.Run("非法引擎", func(t *testing.T) {
		req := domain.Request{Provider: "gitea", Repo: "o/r", PullRequestID: "7", Engine: "copilot"}
		if err := validate(req); err == nil {
			t.Fatal("应拒绝非法 engine")
		}
	})
	t.Run("argus 不支持 commit 模式", func(t *testing.T) {
		req := domain.Request{Provider: "gitea", Repo: "o/r", CommitSHA: "abc1234", Engine: domain.EngineArgus}
		if err := validate(req); err == nil {
			t.Fatal("应拒绝 argus+commit_sha")
		}
	})
	t.Run("合法 argus PR 请求", func(t *testing.T) {
		req := domain.Request{Provider: "gitea", Repo: "o/r", PullRequestID: "7", Engine: domain.EngineArgus}
		if err := validate(req); err != nil {
			t.Fatalf("不应拒绝: %v", err)
		}
	})
	t.Run("task_kind 仅 argus 支持", func(t *testing.T) {
		req := domain.Request{Provider: "gitea", Repo: "o/r", PullRequestID: "7",
			Engine: domain.EngineOCR, TaskKind: domain.TaskKindRequirementCompliance}
		if err := validate(req); err == nil {
			t.Fatal("OCR 请求携带 task_kind 应拒绝")
		}
	})
	t.Run("非法 task_kind 拒绝", func(t *testing.T) {
		req := domain.Request{Provider: "gitea", Repo: "o/r", PullRequestID: "7",
			Engine: domain.EngineArgus, TaskKind: "banana"}
		if err := validate(req); err == nil {
			t.Fatal("应拒绝非法 task_kind")
		}
	})
	t.Run("合法 task_kind 放行", func(t *testing.T) {
		for _, kind := range []string{domain.TaskKindCodeReview, domain.TaskKindRequirementCompliance, domain.TaskKindVisualCheck} {
			req := domain.Request{Provider: "gitea", Repo: "o/r", PullRequestID: "7",
				Engine: domain.EngineArgus, TaskKind: kind}
			if err := validate(req); err != nil {
				t.Fatalf("task_kind=%s 不应拒绝: %v", kind, err)
			}
		}
	})
}

// 触发成功：Record 置 running + dispatch 事件；触发失败：Record 置 failed。
func TestArgusExecutorDispatch(t *testing.T) {
	newReq := func() domain.Request {
		return domain.Request{Provider: "gitea", Repo: "zhangyi/demo", PullRequestID: "7", Engine: domain.EngineArgus}
	}

	t.Run("触发成功置 running", func(t *testing.T) {
		argus := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(`{"code":0,"message":"ok","data":{"taskId":"task-1","status":"pending"}}`))
		}))
		t.Cleanup(argus.Close)
		repo := &fakeReviewRepoForArgus{saved: map[string]*domain.Record{}}
		exec := NewArgusExecutor(argusclient.New(argus.URL, "tok"), repo, nil)

		resp, err := exec.Execute(context.Background(), "rev-1", newReq(), nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		if resp.Status != statusRunning {
			t.Fatalf("响应应为 running: %s", resp.Status)
		}
		rec := repo.saved["rev-1"]
		if rec.Status != statusRunning {
			t.Fatalf("记录应为 running: %s", rec.Status)
		}
		last := rec.Response.History[len(rec.Response.History)-1]
		if last.Stage != "dispatch" || last.Status != "completed" {
			t.Fatalf("dispatch 事件不符: %+v", last)
		}
		if rec.Response.ArgusTaskID != "task-1" {
			t.Fatalf("ArgusTaskID 应落库: %q", rec.Response.ArgusTaskID)
		}
	})

	t.Run("触发失败置 failed", func(t *testing.T) {
		argus := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		t.Cleanup(argus.Close)
		repo := &fakeReviewRepoForArgus{saved: map[string]*domain.Record{}}
		exec := NewArgusExecutor(argusclient.New(argus.URL, "tok"), repo, nil)

		if _, err := exec.Execute(context.Background(), "rev-2", newReq(), nil, nil); err == nil {
			t.Fatal("触发失败应返回错误")
		}
		rec := repo.saved["rev-2"]
		if rec.Status != statusFailed {
			t.Fatalf("记录应为 failed: %s", rec.Status)
		}
	})

	t.Run("repo 非 owner/repo 形式拒绝", func(t *testing.T) {
		repo := &fakeReviewRepoForArgus{saved: map[string]*domain.Record{}}
		exec := NewArgusExecutor(argusclient.New("http://x", "tok"), repo, nil)
		req := newReq()
		req.Repo = "demo"
		if _, err := exec.Execute(context.Background(), "rev-3", req, nil, nil); err == nil {
			t.Fatal("应拒绝裸 repo 名")
		}
		if repo.saved["rev-3"].Status != statusFailed {
			t.Fatal("应落失败终态")
		}
	})
}

// 引擎分流：engine=ocr（含空）转发原执行器；engine=argus 走 argus 执行器；
// argus 未装配时 engine=argus 报错。
func TestEngineExecutorRouting(t *testing.T) {
	noop := &stubExecutor{}
	argusExec := &stubExecutor{}
	ee := NewEngineExecutor(noop, argusExec)

	if _, err := ee.Execute(context.Background(), "r", domain.Request{Engine: ""}, nil, nil); err != nil || !noop.called {
		t.Fatalf("空引擎应走 ocr: called=%v err=%v", noop.called, err)
	}
	if _, err := ee.Execute(context.Background(), "r", domain.Request{Engine: domain.EngineOCR}, nil, nil); err != nil || noop.calls != 2 {
		t.Fatalf("ocr 引擎应走 ocr: calls=%d err=%v", noop.calls, err)
	}
	if _, err := ee.Execute(context.Background(), "r", domain.Request{Engine: domain.EngineArgus}, nil, nil); err != nil || !argusExec.called {
		t.Fatalf("argus 引擎应走 argus: called=%v err=%v", argusExec.called, err)
	}
	if _, err := NewEngineExecutor(noop, nil).Execute(context.Background(), "r", domain.Request{Engine: domain.EngineArgus}, nil, nil); err == nil {
		t.Fatal("argus 未装配应报错")
	}
}

type stubExecutor struct {
	called bool
	calls  int
}

func (s *stubExecutor) Execute(_ context.Context, _ string, _ domain.Request, _ domain.Provider, _ progress.ProgressReporter) (domain.Response, error) {
	s.called = true
	s.calls++
	return domain.Response{}, nil
}
