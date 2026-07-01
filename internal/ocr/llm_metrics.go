package ocr

import (
	"context"
	"sync/atomic"
	"time"
)

// Metrics 统计 LLM 调用的可观测指标。线程安全，零依赖。
type Metrics struct {
	Calls        atomic.Int64
	Successes    atomic.Int64
	Failures     atomic.Int64
	Unavailable  atomic.Int64
	InputTokens  atomic.Int64
	OutputTokens atomic.Int64
	TotalLatency atomic.Int64
}

// MetricsSnapshot 返回当前指标的只读快照。
type MetricsSnapshot struct {
	Calls        int64   `json:"calls"`
	Successes    int64   `json:"successes"`
	Failures     int64   `json:"failures"`
	Unavailable  int64   `json:"unavailable"`
	InputTokens  int64   `json:"input_tokens"`
	OutputTokens int64   `json:"output_tokens"`
	AvgLatencyMs float64 `json:"avg_latency_ms"`
	FailureRate  float64 `json:"failure_rate"`
}

// Snapshot 读取当前指标快照。
func (m *Metrics) Snapshot() MetricsSnapshot {
	calls := m.Calls.Load()
	succ := m.Successes.Load()
	var avgLatency float64
	if calls > 0 {
		avgLatency = float64(m.TotalLatency.Load()) / float64(calls) / float64(time.Millisecond)
	}
	var failRate float64
	if calls > 0 {
		failRate = float64(m.Failures.Load()) / float64(calls)
	}
	return MetricsSnapshot{
		Calls:        calls,
		Successes:    succ,
		Failures:     m.Failures.Load(),
		Unavailable:  m.Unavailable.Load(),
		InputTokens:  m.InputTokens.Load(),
		OutputTokens: m.OutputTokens.Load(),
		AvgLatencyMs: avgLatency,
		FailureRate:  failRate,
	}
}

// MetricsProvider 包装另一个 Provider 并统计调用指标。
type MetricsProvider struct {
	inner Provider
	m     *Metrics
}

// NewMetricsProvider 创建带指标统计的 Provider，返回包装器与指标对象。
func NewMetricsProvider(inner Provider) (*MetricsProvider, *Metrics) {
	if inner == nil {
		inner = NewNoopProvider()
	}
	m := &Metrics{}
	return &MetricsProvider{inner: inner, m: m}, m
}

// Available 委托给内部 Provider，不可用时递增 Unavailable 计数。
func (p *MetricsProvider) Available(ctx context.Context) bool {
	ok := p.inner.Available(ctx)
	if !ok {
		p.m.Unavailable.Add(1)
	}
	return ok
}

// Complete 委托给内部 Provider，统计调用/成功/失败/耗时/token。
func (p *MetricsProvider) Complete(ctx context.Context, req CompleteRequest) (CompleteResponse, error) {
	p.m.Calls.Add(1)
	start := time.Now()
	resp, err := p.inner.Complete(ctx, req)
	p.m.TotalLatency.Add(int64(time.Since(start)))
	if err != nil {
		p.m.Failures.Add(1)
		return resp, err
	}
	p.m.Successes.Add(1)
	p.m.InputTokens.Add(resp.InputTokens)
	p.m.OutputTokens.Add(resp.OutputTokens)
	return resp, nil
}
