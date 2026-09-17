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
	res, err := c.TriggerReview(context.Background(), "Gitea", "zhangyi", "demo", "7")
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
	res, err := c.TriggerReview(context.Background(), "gitea", "zhangyi", "demo", "7")
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
	if _, err := c.TriggerReview(context.Background(), "gitea", "o", "r", "+123"); err == nil {
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
		if _, err := c.TriggerReview(context.Background(), "unknown", "o", "r", "7"); err == nil ||
			!strings.Contains(err.Error(), "bad platform") {
			t.Fatalf("应上抛业务错误: %v", err)
		}
	})
	t.Run("HTTP 500", func(t *testing.T) {
		c := newFakeArgus(t, func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		})
		if _, err := c.TriggerReview(context.Background(), "gitea", "o", "r", "7"); err == nil {
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
	if _, err := c.TriggerReview(context.Background(), "gitea", "o", "r", "7"); err != ErrDisabled {
		t.Fatalf("应返回 ErrDisabled: %v", err)
	}
	if _, err := c.GetTask(context.Background(), "t"); err != ErrDisabled {
		t.Fatalf("应返回 ErrDisabled: %v", err)
	}
	if _, err := c.SuppressFinding(context.Background(), "gitea", "o", "r", "7", "fp"); err != ErrDisabled {
		t.Fatalf("应返回 ErrDisabled: %v", err)
	}
}
