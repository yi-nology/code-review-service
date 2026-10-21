package review

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/yi-nology/code-review-service/internal/argusclient"
	"github.com/yi-nology/code-review-service/internal/domain/review"
)

// argus 任务失败（不回推）→ 轮询器把 running 记录收敛 failed；
// done 但记录仍 running（sink 丢失）→ 拉全量详情恢复落库并收敛 verdict 终态，
// 拉取失败才降级 failed；running → 不动。
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
		p := NewArgusPoller(repo, &fakeArgusReviewRepo{}, argusclient.New(argus.URL, "tok"), nil, nil, time.Second)
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

	t.Run("done 但 sink 丢失 → 拉取恢复收敛 verdict 终态", func(t *testing.T) {
		argus := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/tasks/t-2" {
				_, _ = w.Write([]byte(`{"code":0,"message":"ok","data":{"taskId":"t-2","status":"done","verdict":"request_changes",
					"report":"# 报告","findings":[{"file":"a.go","line":3,"severity":"high","comment":"空指针","by":"correctness"}]}}`))
				return
			}
			_, _ = w.Write([]byte(`{"code":0,"message":"ok","data":{"taskId":"t-2","status":"done","verdict":"request_changes"}}`))
		}))
		t.Cleanup(argus.Close)
		repo := &fakeReviewRepoForArgus{saved: map[string]*domain.Record{}}
		repo.saved["rev-d"] = &domain.Record{
			ID: "rev-d", Status: statusRunning, HeadSHA: "sha-d",
			Request:  domain.Request{Engine: domain.EngineArgus, Provider: "gitea", Repo: "o/r", PullRequestID: "7"},
			Response: domain.Response{ArgusTaskID: "t-2"},
		}
		argusRepo := &fakeArgusReviewRepo{}
		p := NewArgusPoller(repo, argusRepo, argusclient.New(argus.URL, "tok"), nil, nil, time.Second)
		if err := p.sweep(context.Background()); err != nil {
			t.Fatal(err)
		}
		rec := repo.saved["rev-d"]
		if rec.Status != statusSuccess {
			t.Fatalf("恢复后应收敛 success（request_changes → gate failed 但非 failed 终态）: %s", rec.Status)
		}
		if rec.Response.Gate.Status != domain.GateFailed {
			t.Fatalf("request_changes 应 gate failed: %+v", rec.Response.Gate)
		}
		if len(rec.Response.Findings) != 1 || rec.Response.Findings[0].Path != "a.go" {
			t.Fatalf("恢复的 findings 应写入评审记录: %+v", rec.Response.Findings)
		}
		if len(argusRepo.upserts) != 1 || argusRepo.upserts[0].HeadSHA != "sha-d" ||
			argusRepo.upserts[0].FindingsTotal != 1 {
			t.Fatalf("恢复结果应落承接表: %+v", argusRepo.upserts)
		}
	})

	t.Run("done 但承接表落库失败 → 降级 failed", func(t *testing.T) {
		// GetTask 与 GetTaskDetail 同路径，无法只让详情失败；用承接表 upsert
		// 失败走同一降级出口（markReportLost）。
		argus := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(`{"code":0,"message":"ok","data":{"taskId":"t-2b","status":"done","verdict":"approve",
				"findings":[{"file":"a.go","line":1,"severity":"low","comment":"x"}]}}`))
		}))
		t.Cleanup(argus.Close)
		repo := &fakeReviewRepoForArgus{saved: map[string]*domain.Record{}}
		repo.saved["rev-d2"] = &domain.Record{
			ID: "rev-d2", Status: statusRunning,
			Request:  domain.Request{Engine: domain.EngineArgus},
			Response: domain.Response{ArgusTaskID: "t-2b"},
		}
		p := NewArgusPoller(repo, &fakeArgusReviewRepo{fail: true}, argusclient.New(argus.URL, "tok"), nil, nil, time.Second)
		if err := p.sweep(context.Background()); err != nil {
			t.Fatal(err)
		}
		if repo.saved["rev-d2"].Status != statusFailed {
			t.Fatalf("恢复失败应降级 failed: %s", repo.saved["rev-d2"].Status)
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
		p := NewArgusPoller(repo, &fakeArgusReviewRepo{}, argusclient.New(argus.URL, "tok"), nil, nil, time.Second)
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
		p := NewArgusPoller(repo, &fakeArgusReviewRepo{}, argusclient.New(argus.URL, "tok"), nil, nil, time.Second)
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
