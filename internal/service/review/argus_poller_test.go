package review

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/yi-nology/code-review-service/internal/argusclient"
	"github.com/yi-nology/code-review-service/internal/service/review/domain"
)

// argus 任务失败（不回推）→ 轮询器把 running 记录收敛 failed；
// done 但记录仍 running（sink 丢失）→ 收敛 failed 并提示看 Argus 页；
// running → 不动。
func TestArgusPollerSweep(t *testing.T) {
	t.Run("task failed 收敛", func(t *testing.T) {
		argus := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{"code":0,"message":"ok","data":{"taskId":"t-1","status":"failed","failReason":"回帖失败: token scope"}}`))
		}))
		t.Cleanup(argus.Close)
		repo := &fakeReviewRepoForArgus{saved: map[string]*domain.Record{}}
		repo.saved["rev-f"] = &domain.Record{
			ID: "rev-f", Status: statusRunning,
			Request:  domain.Request{Engine: domain.EngineArgus},
			Response: domain.Response{ArgusTaskID: "t-1"},
		}
		p := NewArgusPoller(repo, argusclient.New(argus.URL, "tok"), nil, time.Second)
		if err := p.sweep(context.Background()); err != nil {
			t.Fatal(err)
		}
		rec := repo.saved["rev-f"]
		if rec.Status != statusFailed {
			t.Fatalf("应收敛 failed: %s", rec.Status)
		}
		last := rec.Response.History[len(rec.Response.History)-1]
		if last.Stage != "argus_poll" {
			t.Fatalf("应有 argus_poll 事件: %+v", last)
		}
	})

	t.Run("done 但 sink 丢失", func(t *testing.T) {
		argus := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(`{"code":0,"message":"ok","data":{"taskId":"t-2","status":"done","verdict":"approve"}}`))
		}))
		t.Cleanup(argus.Close)
		repo := &fakeReviewRepoForArgus{saved: map[string]*domain.Record{}}
		repo.saved["rev-d"] = &domain.Record{
			ID: "rev-d", Status: statusRunning,
			Request:  domain.Request{Engine: domain.EngineArgus},
			Response: domain.Response{ArgusTaskID: "t-2"},
		}
		p := NewArgusPoller(repo, argusclient.New(argus.URL, "tok"), nil, time.Second)
		if err := p.sweep(context.Background()); err != nil {
			t.Fatal(err)
		}
		if repo.saved["rev-d"].Status != statusFailed {
			t.Fatalf("sink 丢失应收敛 failed: %s", repo.saved["rev-d"].Status)
		}
	})

	t.Run("running 与 ocr 引擎不动", func(t *testing.T) {
		argus := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(`{"code":0,"message":"ok","data":{"taskId":"t-3","status":"running"}}`))
		}))
		t.Cleanup(argus.Close)
		repo := &fakeReviewRepoForArgus{saved: map[string]*domain.Record{}}
		repo.saved["rev-r"] = &domain.Record{
			ID: "rev-r", Status: statusRunning,
			Request:  domain.Request{Engine: domain.EngineArgus},
			Response: domain.Response{ArgusTaskID: "t-3"},
		}
		repo.saved["rev-o"] = &domain.Record{
			ID: "rev-o", Status: statusRunning,
			Request: domain.Request{Engine: domain.EngineOCR},
		}
		p := NewArgusPoller(repo, argusclient.New(argus.URL, "tok"), nil, time.Second)
		if err := p.sweep(context.Background()); err != nil {
			t.Fatal(err)
		}
		if repo.saved["rev-r"].Status != statusRunning {
			t.Fatalf("running 不应被动: %s", repo.saved["rev-r"].Status)
		}
		if repo.saved["rev-o"].Status != statusRunning {
			t.Fatalf("ocr 引擎不应被动: %s", repo.saved["rev-o"].Status)
		}
	})

	t.Run("已收敛的记录不覆盖", func(t *testing.T) {
		argus := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(`{"code":0,"message":"ok","data":{"taskId":"t-4","status":"failed"}}`))
		}))
		t.Cleanup(argus.Close)
		repo := &fakeReviewRepoForArgus{saved: map[string]*domain.Record{}}
		repo.saved["rev-x"] = &domain.Record{
			ID: "rev-x", Status: statusRunning,
			Request:  domain.Request{Engine: domain.EngineArgus},
			Response: domain.Response{ArgusTaskID: "t-4"},
		}
		p := NewArgusPoller(repo, argusclient.New(argus.URL, "tok"), nil, time.Second)
		// 轮询期间 sink 回推先到：UpdateInTransaction 里二次校验 running。
		// 模拟：先手动把状态改成 success（模拟并发收敛），轮询更新应跳过。
		repo.saved["rev-x"].Status = statusSuccess
		// 注：sweep 快照里仍是 running（List 时刻的值），UpdateInTransaction
		// 收到的 fn 会看到已收敛的记录——验证不覆盖语义。
		if err := p.sweep(context.Background()); err != nil {
			t.Fatal(err)
		}
		if repo.saved["rev-x"].Status != statusSuccess {
			t.Fatalf("已收敛记录不应被覆盖: %s", repo.saved["rev-x"].Status)
		}
	})
}
