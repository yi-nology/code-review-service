package argusclient

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// envelope 包一层 ekit 信封，模拟 argus 侧 mw.SuccessRespMsg 的真实响应形态。
func envelope(data string) string {
	return `{"code":0,"message":"ok","data":` + data + `}`
}

func newFakeArgus(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return New(srv.URL, "tok")
}

// 触发成功：POST /tasks 带业务键 + Bearer，kind=1，返回 taskId/status。
func TestTriggerReviewOK(t *testing.T) {
	var gotPath, gotAuth, gotBody string
	c := newFakeArgus(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotAuth = r.URL.Path, r.Header.Get("Authorization")
		buf := make([]byte, r.ContentLength)
		_, _ = r.Body.Read(buf)
		gotBody = string(buf)
		_, _ = w.Write([]byte(envelope(`{"taskId":"task-1","status":"pending"}`)))
	})
	res, err := c.TriggerTask(context.Background(), TriggerOptions{Platform: "Gitea", Owner: "zhangyi", Repo: "demo", Number: "7", Kind: TaskKindCodeReview})
	if err != nil {
		t.Fatal(err)
	}
	if gotPath != "/tasks" || gotAuth != "Bearer tok" {
		t.Fatalf("path=%q auth=%q", gotPath, gotAuth)
	}
	if !strings.Contains(gotBody, `"kind":1`) || !strings.Contains(gotBody, `"platform":"gitea"`) ||
		!strings.Contains(gotBody, `"owner":"zhangyi"`) || !strings.Contains(gotBody, `"number":"7"`) {
		t.Fatalf("请求体不符: %s", gotBody)
	}
	if res.TaskID != "task-1" || res.Status != "pending" || res.Duplicated {
		t.Fatalf("结果不符: %+v", res)
	}
}

// 同 PR 在飞任务：argus 幂等返回既有任务并带 duplicated=true。
func TestTriggerReviewDuplicated(t *testing.T) {
	c := newFakeArgus(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(envelope(`{"taskId":"task-existing","status":"running","duplicated":true}`)))
	})
	res, err := c.TriggerTask(context.Background(), TriggerOptions{Platform: "gitea", Owner: "zhangyi", Repo: "demo", Number: "7", Kind: TaskKindCodeReview})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Duplicated || res.TaskID != "task-existing" {
		t.Fatalf("应识别 duplicated: %+v", res)
	}
}

// number 非纯数字在客户端即拒绝，不触达 argus。
func TestTriggerReviewBadNumber(t *testing.T) {
	called := false
	c := newFakeArgus(t, func(w http.ResponseWriter, _ *http.Request) {
		called = true
		_, _ = w.Write([]byte(envelope(`{}`)))
	})
	if _, err := c.TriggerTask(context.Background(), TriggerOptions{Platform: "gitea", Owner: "o", Repo: "r", Number: "+123", Kind: TaskKindCodeReview}); err == nil {
		t.Fatal("应拒绝非纯数字 number")
	}
	if called {
		t.Fatal("不应触达 argus")
	}
}

// 业务错误（信封 code!=0）与 HTTP 错误均上抛。
func TestTriggerReviewErrors(t *testing.T) {
	t.Run("信封 code!=0", func(t *testing.T) {
		c := newFakeArgus(t, func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(`{"code":1,"message":"bad platform"}`))
		})
		if _, err := c.TriggerTask(context.Background(), TriggerOptions{Platform: "unknown", Owner: "o", Repo: "r", Number: "7"}); err == nil ||
			!strings.Contains(err.Error(), "bad platform") {
			t.Fatalf("应上抛业务错误: %v", err)
		}
	})
	t.Run("HTTP 500", func(t *testing.T) {
		c := newFakeArgus(t, func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		})
		if _, err := c.TriggerTask(context.Background(), TriggerOptions{Platform: "gitea", Owner: "o", Repo: "r", Number: "7", Kind: TaskKindCodeReview}); err == nil {
			t.Fatal("应上抛 HTTP 错误")
		}
	})
}

// GetTask 解析 status/verdict/failReason。
func TestGetTask(t *testing.T) {
	c := newFakeArgus(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/tasks/task-9" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_, _ = w.Write([]byte(envelope(`{"taskId":"task-9","status":"failed","failReason":"llm timeout"}`)))
	})
	st, err := c.GetTask(context.Background(), "task-9")
	if err != nil {
		t.Fatal(err)
	}
	if st.Status != "failed" || st.FailReason != "llm timeout" {
		t.Fatalf("状态不符: %+v", st)
	}
}

// Suppress：命中/未命中两态。
func TestSuppressFinding(t *testing.T) {
	t.Run("matched", func(t *testing.T) {
		c := newFakeArgus(t, func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(envelope(`{"matched":true,"fingerprint":"fp-full"}`)))
		})
		matched, err := c.SuppressFinding(context.Background(), "gitea", "o", "r", "7", "fp")
		if err != nil || !matched {
			t.Fatalf("matched=%v err=%v", matched, err)
		}
	})
	t.Run("unmatched", func(t *testing.T) {
		c := newFakeArgus(t, func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(envelope(`{"matched":false,"fingerprint":""}`)))
		})
		matched, err := c.SuppressFinding(context.Background(), "gitea", "o", "r", "7", "fp")
		if err != nil || matched {
			t.Fatalf("matched=%v err=%v", matched, err)
		}
	})
}

// 未配置（bot_url 空）时 Enabled=false 且所有调用返回 ErrDisabled。
func TestDisabled(t *testing.T) {
	c := New("", "")
	if c.Enabled() {
		t.Fatal("空 baseURL 应 disabled")
	}
	if _, err := c.TriggerTask(context.Background(), TriggerOptions{Platform: "gitea", Owner: "o", Repo: "r", Number: "7", Kind: TaskKindCodeReview}); err != ErrDisabled {
		t.Fatalf("应返回 ErrDisabled: %v", err)
	}
	if _, err := c.GetTask(context.Background(), "t"); err != ErrDisabled {
		t.Fatalf("应返回 ErrDisabled: %v", err)
	}
	if _, err := c.SuppressFinding(context.Background(), "gitea", "o", "r", "7", "fp"); err != ErrDisabled {
		t.Fatalf("应返回 ErrDisabled: %v", err)
	}
}

// SupportsPlatform 平台白名单：argus 只接 gitea/github/gitlab。
func TestSupportsPlatform(t *testing.T) {
	for _, p := range []string{"gitea", "GitHub", " gitlab "} {
		if !SupportsPlatform(p) {
			t.Fatalf("%s 应在支持面内", p)
		}
	}
	for _, p := range []string{"gerrit", "gitcode", "tencent_code", ""} {
		if SupportsPlatform(p) {
			t.Fatalf("%s 不应在支持面内", p)
		}
	}
}

// TriggerTask 携带任务类型与 artifacts。
func TestTriggerTaskKindAndArtifacts(t *testing.T) {
	var gotBody string
	c := newFakeArgus(t, func(w http.ResponseWriter, r *http.Request) {
		buf := make([]byte, r.ContentLength)
		_, _ = r.Body.Read(buf)
		gotBody = string(buf)
		_, _ = w.Write([]byte(envelope(`{"taskId":"task-r1","status":"pending"}`)))
	})
	res, err := c.TriggerTask(context.Background(), TriggerOptions{
		Platform: "gitea", Owner: "o", Repo: "r", Number: "7",
		Kind:      TaskKindRequirementCompliance,
		Artifacts: map[string]string{"requirements": "AC1\nAC2"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.TaskID != "task-r1" {
		t.Fatalf("结果不符: %+v", res)
	}
	if !strings.Contains(gotBody, `"kind":0`) || !strings.Contains(gotBody, `"artifacts"`) ||
		!strings.Contains(gotBody, "AC1") {
		t.Fatalf("请求体应含 kind=0 与 artifacts: %s", gotBody)
	}
	// 非法 kind 客户端即拒绝。
	if _, err := c.TriggerTask(context.Background(), TriggerOptions{Platform: "gitea", Owner: "o", Repo: "r", Number: "7", Kind: 9}); err == nil {
		t.Fatal("应拒绝非法 kind")
	}
}

// GetTaskDetail 解析全量任务详情（findings + report）。
func TestGetTaskDetail(t *testing.T) {
	c := newFakeArgus(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/tasks/task-9" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_, _ = w.Write([]byte(envelope(`{"taskId":"task-9","status":"done","verdict":"comment",
			"report":"# 报告全文","reportRef":"abc123",
			"findings":[{"file":"a.go","line":3,"severity":"high","comment":"空指针","by":"correctness"}]}`)))
	})
	d, err := c.GetTaskDetail(context.Background(), "task-9")
	if err != nil {
		t.Fatal(err)
	}
	if d.Status != "done" || d.Verdict != "comment" || d.Report != "# 报告全文" || d.ReportRef != "abc123" {
		t.Fatalf("详情不符: %+v", d)
	}
	if len(d.Findings) != 1 || d.Findings[0].File != "a.go" || d.Findings[0].Line != 3 ||
		d.Findings[0].Severity != "high" || d.Findings[0].By != "correctness" {
		t.Fatalf("findings 不符: %+v", d.Findings)
	}
}

// ListTasks 解析任务列表（含字符串 kind 与 headSha）。
func TestListTasks(t *testing.T) {
	c := newFakeArgus(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.RawQuery != "status=running&limit=50" {
			t.Errorf("query 不符: %s", r.URL.RawQuery)
		}
		_, _ = w.Write([]byte(envelope(`{"tasks":[{"taskId":"t1","status":"running","kind":"code_review",
			"platform":"gitea","owner":"o","repo":"r","number":"7","headSha":"abc","createdAt":1726500000}]}`)))
	})
	tasks, err := c.ListTasks(context.Background(), "running", 50)
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 1 || tasks[0].TaskID != "t1" || tasks[0].Kind != "code_review" ||
		tasks[0].HeadSHA != "abc" || tasks[0].CreatedAt != 1726500000 {
		t.Fatalf("列表不符: %+v", tasks)
	}
}

// GetUsageMonthly 解析本月用量汇总。
func TestGetUsageMonthly(t *testing.T) {
	c := newFakeArgus(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(envelope(`{"month":"2026-09","total":1234,"budget":5000,
			"repos":[{"platform":"gitea","owner":"o","repo":"r","tokens":1234,"tasks":3}]}`)))
	})
	u, err := c.GetUsageMonthly(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if u.Month != "2026-09" || u.Total != 1234 || u.Budget != 5000 || len(u.Repos) != 1 || u.Repos[0].Tokens != 1234 {
		t.Fatalf("用量不符: %+v", u)
	}
}

// GetReportHealth 透传原始 JSON。
func TestGetReportHealth(t *testing.T) {
	c := newFakeArgus(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(envelope(`{"generated_at":"2026-09-18T00:00:00Z","repos":[],"empty":true}`)))
	})
	raw, err := c.GetReportHealth(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"empty":true`) {
		t.Fatalf("应透传原始 JSON: %s", raw)
	}
}

// SuppressFindingByComment 走 file+comment 兜底模式。
func TestSuppressFindingByComment(t *testing.T) {
	var gotBody string
	c := newFakeArgus(t, func(w http.ResponseWriter, r *http.Request) {
		buf := make([]byte, r.ContentLength)
		_, _ = r.Body.Read(buf)
		gotBody = string(buf)
		_, _ = w.Write([]byte(envelope(`{"matched":true,"fingerprint":"fp-derived"}`)))
	})
	matched, err := c.SuppressFindingByComment(context.Background(), "gitea", "o", "r", "7", "a.go", "意见原文")
	if err != nil || !matched {
		t.Fatalf("matched=%v err=%v", matched, err)
	}
	if !strings.Contains(gotBody, `"file":"a.go"`) || !strings.Contains(gotBody, `"comment"`) ||
		strings.Contains(gotBody, "fingerprint") {
		t.Fatalf("应携带 file+comment 且不带 fingerprint: %s", gotBody)
	}
}
