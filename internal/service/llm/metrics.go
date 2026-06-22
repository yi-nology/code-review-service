package llm

import (
	"context"
	"sync/atomic"
	"time"
)

// Metrics 统计 LLM 调用的可观测指标（阶段 6）。线程安全，零依赖。
// 所有计数用 atomic，可在 prometheus/日志/调试端点中直接快照读取。
type Metrics struct {
	Calls        atomic.Int64 // 总 Complete 调用次数
	Successes    atomic.Int64 // 成功次数
	Failures     atomic.Int64 // 失败次数（含超时、HTTP 错误、解析失败）
	Unavailable  atomic.Int64 // Available 返回 false 的次数（降级触发点）
	InputTokens  atomic.Int64 // 累计输入 token
	OutputTokens atomic.Int64 // 累计输出 token
	TotalLatency atomic.Int64 // 累计耗时（纳秒）
}

// Snapshot 返回当前指标的只读快照。
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

// MetricsProvider 包装另一个 Provider 并统计调用指标（阶段 6）。
// 通过包装 DynamicProvider 实现：所有 LLM 调用必经此处，可统一观测。
type MetricsProvider struct {
	inner Provider
	m     *Metrics
}

// NewMetricsProvider 创建带指标统计的 Provider，返回包装器与指标对象。
// 指标对象可注入 handler/stats 供运行时观测。
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
