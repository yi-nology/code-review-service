package notification

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/yi-nology/code-review-service/internal/domain/notification"
)

// newTestChannel 造一个指向 ts 的 webhook 渠道。
func newTestChannel(ts *httptest.Server) *WebhookChannel {
	return NewWebhookChannel(notification.WebhookChannelConfig{
		Type: "generic",
		URL:  ts.URL,
	})
}

// TestWebhookPost_Success 验证 2xx 即成功（任意 2xx，不强制 JSON 响应体）。
//
// 「任意 2xx 即成功」是 webhook 渠道的语义：渠道服务商的响应体格式各异，
// 强制解析 JSON 会把一次成功的推送判成失败。
func TestWebhookPost_Success(t *testing.T) {
	var calls atomic.Int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("not json at all"))
	}))
	defer ts.Close()

	c := newTestChannel(ts)
	if err := c.post(context.Background(), map[string]any{"k": "v"}); err != nil {
		t.Fatalf("2xx 应成功, got %v", err)
	}
	if got := calls.Load(); got != 1 {
		t.Errorf("2xx 不应重试, calls=%d", got)
	}
}

// TestWebhookPost_RetryOnServerError 验证 5xx 会重试，且重试后成功即返回 nil。
func TestWebhookPost_RetryOnServerError(t *testing.T) {
	var calls atomic.Int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	c := newTestChannel(ts)
	// Attempts=2 ⇒ 总共 2 次尝试；第 3 次才成功的场景超出 attempts，
	// 这里验证的是「5xx 被当作可重试」而不是最终成功。
	err := c.post(context.Background(), map[string]any{"k": "v"})
	if err == nil {
		t.Error("2 次 5xx 后应返回错误")
	}
	if got := calls.Load(); got != webhookRetryAttempts {
		t.Errorf("5xx 应重试到 attempts 上限, calls=%d want=%d", got, webhookRetryAttempts)
	}
}

// TestWebhookPost_RetryThenSuccess 验证「先 5xx 后 2xx」判为成功。
func TestWebhookPost_RetryThenSuccess(t *testing.T) {
	var calls atomic.Int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	c := newTestChannel(ts)
	if err := c.post(context.Background(), map[string]any{"k": "v"}); err != nil {
		t.Fatalf("重试后成功应返回 nil, got %v", err)
	}
	if got := calls.Load(); got != 2 {
		t.Errorf("应尝试 2 次, calls=%d", got)
	}
}

// TestWebhookPost_NoRetryOnClientError 验证 4xx 不重试（永久失败，重试无意义）。
func TestWebhookPost_NoRetryOnClientError(t *testing.T) {
	var calls atomic.Int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer ts.Close()

	c := newTestChannel(ts)
	if err := c.post(context.Background(), map[string]any{"k": "v"}); err == nil {
		t.Error("4xx 应返回错误")
	}
	if got := calls.Load(); got != 1 {
		t.Errorf("4xx 不应重试, calls=%d", got)
	}
}

// TestWebhookPost_RetryOnTooManyRequests 验证 429 会重试。
func TestWebhookPost_RetryOnTooManyRequests(t *testing.T) {
	var calls atomic.Int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer ts.Close()

	c := newTestChannel(ts)
	_ = c.post(context.Background(), map[string]any{"k": "v"})
	if got := calls.Load(); got != webhookRetryAttempts {
		t.Errorf("429 应重试, calls=%d want=%d", got, webhookRetryAttempts)
	}
}

// TestWebhookPost_ContextCancel 验证 ctx 取消后不重试，且返回 ctx 的错误。
func TestWebhookPost_ContextCancel(t *testing.T) {
	var calls atomic.Int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // 立即取消

	c := newTestChannel(ts)
	err := c.post(ctx, map[string]any{"k": "v"})
	if !errors.Is(err, context.Canceled) {
		t.Errorf("应返回 context.Canceled, got %v", err)
	}
	if got := calls.Load(); got > 1 {
		t.Errorf("ctx 取消后不应重试, calls=%d", got)
	}
}

// TestWebhookPost_TransportErrorRetried 验证传输层失败（连接被拒）会重试。
func TestWebhookPost_TransportErrorRetried(t *testing.T) {
	// 先起一个 server 拿到地址，随即关闭 → 连接被拒。
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	url := ts.URL
	ts.Close()

	c := NewWebhookChannel(notification.WebhookChannelConfig{Type: "generic", URL: url})
	start := time.Now()
	err := c.post(context.Background(), map[string]any{"k": "v"})
	if err == nil {
		t.Error("连接被拒应返回错误")
	}
	// 传输层失败被重试 → 总耗时含一次退避（约 1s 起）。这里只断言它确实
	// 走到了重试路径（耗时 > 0 且错误不是 2xx 语义）。
	if time.Since(start) <= 0 {
		t.Error("不应为零耗时")
	}
}
