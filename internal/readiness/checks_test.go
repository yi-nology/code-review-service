package readiness

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// fakeDial 可编排拨号替身：按调用次序返回预设结果。
type fakeDial struct {
	results []error
	calls   int
	closes  int
}

func (f *fakeDial) dial(context.Context, string) (net.Conn, error) {
	i := f.calls
	f.calls++
	if i >= len(f.results) || f.results[i] != nil {
		var err error
		if i < len(f.results) {
			err = f.results[i]
		} else {
			err = errors.New("exhausted")
		}
		return nil, err
	}
	f.closes++
	return stubConn{}, nil
}

// stubConn 空连接（dialWithFlapTolerance 只 Close，不读写）。
type stubConn struct{ net.Conn }

func (stubConn) Close() error { return nil }

func withFastRetry(t *testing.T) {
	t.Helper()
	old := flapRetryDelay
	flapRetryDelay = time.Millisecond
	t.Cleanup(func() { flapRetryDelay = old })
}

// TestDialFlapTolerance_RecoversOnSecondProbe 抖动豁免核心行为：首探失败、
// 复探成功 = 端点可达（单次抖动不打红）。
func TestDialFlapTolerance_RecoversOnSecondProbe(t *testing.T) {
	withFastRetry(t)
	f := &fakeDial{results: []error{errors.New("dns timeout"), nil}}
	err := dialWithFlapTolerance(context.Background(), "x:443", f.dial)
	if err != nil {
		t.Fatalf("抖动后复探成功应判可达: %v", err)
	}
	if f.calls != 2 {
		t.Fatalf("应恰好两探，got %d", f.calls)
	}
}

// TestDialFlapTolerance_BothFailReportsTwice 两次全失败才报错（防单次打红的
// 对偶约束：真故障不被豁免吃掉）。
func TestDialFlapTolerance_BothFailReportsTwice(t *testing.T) {
	withFastRetry(t)
	f := &fakeDial{results: []error{errors.New("refused"), errors.New("refused")}}
	err := dialWithFlapTolerance(context.Background(), "x:443", f.dial)
	if err == nil || !strings.Contains(err.Error(), "连探 2 次") {
		t.Fatalf("两次全失败应报错且注明双探: %v", err)
	}
}

// TestDialFlapTolerance_FirstSuccessSingleProbe 首探成功不复探（零抖动路径）。
func TestDialFlapTolerance_FirstSuccessSingleProbe(t *testing.T) {
	withFastRetry(t)
	f := &fakeDial{results: []error{nil}}
	if err := dialWithFlapTolerance(context.Background(), "x:443", f.dial); err != nil {
		t.Fatalf("首探成功应可达: %v", err)
	}
	if f.calls != 1 {
		t.Fatalf("首探成功不应复探，got %d 次", f.calls)
	}
}

// TestDialFlapTolerance_CtxCancelDuringBackoff 退避等待中 ctx 取消立即返回。
func TestDialFlapTolerance_CtxCancelDuringBackoff(t *testing.T) {
	old := flapRetryDelay
	flapRetryDelay = time.Second
	t.Cleanup(func() { flapRetryDelay = old })
	ctx, cancel := context.WithCancel(context.Background())
	f := &fakeDial{results: []error{errors.New("flap")}}
	go func() { time.Sleep(10 * time.Millisecond); cancel() }()
	err := dialWithFlapTolerance(ctx, "x:443", f.dial)
	if err == nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("ctx 取消应经错误链透出: %v", err)
	}
	if f.calls != 1 {
		t.Fatalf("取消后不应复探，got %d", f.calls)
	}
}

// TestProbeLLMUsability_Classifies 可用性探测三态分类（v3.179.1）：
// 200=可用 / 429=配额尽语义 / 401=凭证面 / 5xx=端点故障——
// 「可达≠可用」盲区修复的行为钉。
func TestProbeLLMUsability_Classifies(t *testing.T) {
	cases := []struct {
		status  int
		wantErr string
	}{
		{200, ""},
		{429, "配额尽"},
		{401, "凭证失效"},
		{500, "HTTP 500"},
	}
	for _, c := range cases {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(c.status)
		}))
		err := probeLLMUsability(context.Background(), srv.URL, "k", "", "m")
		if c.wantErr == "" && err != nil {
			t.Fatalf("200 应可用: %v", err)
		}
		if c.wantErr != "" && (err == nil || !strings.Contains(err.Error(), c.wantErr)) {
			t.Fatalf("status %d 应报含 %q 的错误: %v", c.status, c.wantErr, err)
		}
		srv.Close()
	}
}

// TestLLMAvailability_CacheThrottle 缓存节流：两次探测请求只发一次 LLM 调用
// （防 /health 轮询放大成本）。
func TestLLMAvailability_CacheThrottle(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.WriteHeader(200)
	}))
	defer srv.Close()
	llmUsability.mu.Lock()
	llmUsability.at, llmUsability.lastErr = time.Time{}, nil
	llmUsability.mu.Unlock()
	probe := func() error { return probeLLMUsability(context.Background(), srv.URL, "k", "", "m") }
	_ = llmUsability.probe(probe)
	_ = llmUsability.probe(probe)
	if n := atomic.LoadInt32(&hits); n != 1 {
		t.Fatalf("缓存 TTL 内应只发 1 次 LLM 探测，got %d", n)
	}
}
