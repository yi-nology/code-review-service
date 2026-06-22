package llm

import (
	"context"
	"errors"
	"testing"
)

// stubProvider 是测试用的可控 Provider。
type stubProvider struct {
	available bool
	resp      CompleteResponse
	err       error
}

func (s stubProvider) Available(_ context.Context) bool { return s.available }
func (s stubProvider) Complete(_ context.Context, _ CompleteRequest) (CompleteResponse, error) {
	if s.err != nil {
		return CompleteResponse{}, s.err
	}
	return s.resp, nil
}

func TestMetricsProviderCountsSuccess(t *testing.T) {
	inner := stubProvider{
		available: true,
		resp:      CompleteResponse{Content: "ok", InputTokens: 10, OutputTokens: 5},
	}
	mp, m := NewMetricsProvider(inner)

	if !mp.Available(context.Background()) {
		t.Fatal("should be available")
	}
	// Available=true 不递增 Unavailable
	if m.Unavailable.Load() != 0 {
		t.Fatalf("Unavailable should be 0, got %d", m.Unavailable.Load())
	}

	if _, err := mp.Complete(context.Background(), CompleteRequest{}); err != nil {
		t.Fatal(err)
	}

	if m.Calls.Load() != 1 || m.Successes.Load() != 1 || m.Failures.Load() != 0 {
		t.Fatalf("counts wrong: calls=%d succ=%d fail=%d", m.Calls.Load(), m.Successes.Load(), m.Failures.Load())
	}
	if m.InputTokens.Load() != 10 || m.OutputTokens.Load() != 5 {
		t.Fatalf("token counts wrong: in=%d out=%d", m.InputTokens.Load(), m.OutputTokens.Load())
	}
}

func TestMetricsProviderCountsFailure(t *testing.T) {
	inner := stubProvider{available: true, err: errors.New("boom")}
	mp, m := NewMetricsProvider(inner)

	if _, err := mp.Complete(context.Background(), CompleteRequest{}); err == nil {
		t.Fatal("expected error")
	}
	if m.Failures.Load() != 1 || m.Successes.Load() != 0 {
		t.Fatalf("failure count wrong: fail=%d succ=%d", m.Failures.Load(), m.Successes.Load())
	}

	snap := m.Snapshot()
	if snap.FailureRate != 1.0 {
		t.Fatalf("failure rate should be 1.0, got %f", snap.FailureRate)
	}
}

func TestMetricsProviderCountsUnavailable(t *testing.T) {
	inner := stubProvider{available: false}
	mp, m := NewMetricsProvider(inner)

	if mp.Available(context.Background()) {
		t.Fatal("should be unavailable")
	}
	if m.Unavailable.Load() != 1 {
		t.Fatalf("Unavailable should be 1, got %d", m.Unavailable.Load())
	}

	snap := m.Snapshot()
	if snap.Unavailable != 1 {
		t.Fatalf("snapshot Unavailable wrong: %d", snap.Unavailable)
	}
}

func TestMetricsProviderNilInnerDegradesGracefully(t *testing.T) {
	mp, m := NewMetricsProvider(nil) // 内部用 NoopProvider
	if mp.Available(context.Background()) {
		t.Fatal("nil inner should be unavailable")
	}
	if _, err := mp.Complete(context.Background(), CompleteRequest{}); !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("expected ErrNotConfigured, got %v", err)
	}
	if m.Failures.Load() != 1 {
		t.Fatalf("failure should be counted, got %d", m.Failures.Load())
	}
}
