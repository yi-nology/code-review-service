package httpx

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestDoWithRetry_Success(t *testing.T) {
	calls := 0
	err := DoWithRetry(context.Background(), RetryConfig{MaxAttempts: 3}, func(ctx context.Context) (bool, error) {
		calls++
		return false, nil
	})
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	if calls != 1 {
		t.Fatalf("expected 1 call, got %d", calls)
	}
}

func TestDoWithRetry_RetryThenSuccess(t *testing.T) {
	calls := 0
	err := DoWithRetry(context.Background(), RetryConfig{MaxAttempts: 3}, func(ctx context.Context) (bool, error) {
		calls++
		if calls < 3 {
			return true, errors.New("transient")
		}
		return false, nil
	})
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	if calls != 3 {
		t.Fatalf("expected 3 calls, got %d", calls)
	}
}

func TestDoWithRetry_ExhaustAttempts(t *testing.T) {
	calls := 0
	transient := errors.New("transient")
	err := DoWithRetry(context.Background(), RetryConfig{
		MaxAttempts: 3,
		BaseBackoff: 1 * time.Millisecond,
		Jitter:      0,
	}, func(ctx context.Context) (bool, error) {
		calls++
		return true, transient
	})
	if !errors.Is(err, transient) {
		t.Fatalf("expected transient, got %v", err)
	}
	if calls != 3 {
		t.Fatalf("expected 3 calls, got %d", calls)
	}
}

func TestDoWithRetry_NoRetryOnPermanent(t *testing.T) {
	calls := 0
	permanent := errors.New("permanent")
	err := DoWithRetry(context.Background(), RetryConfig{MaxAttempts: 5}, func(ctx context.Context) (bool, error) {
		calls++
		return false, permanent
	})
	if !errors.Is(err, permanent) {
		t.Fatalf("expected permanent, got %v", err)
	}
	if calls != 1 {
		t.Fatalf("expected 1 call (no retry on permanent), got %d", calls)
	}
}

func TestDoWithRetry_Defaults(t *testing.T) {
	calls := 0
	err := DoWithRetry(context.Background(), RetryConfig{}, func(ctx context.Context) (bool, error) {
		calls++
		return true, errors.New("x")
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if calls != 2 {
		t.Fatalf("expected 2 calls (default MaxAttempts), got %d", calls)
	}
}

func TestDoWithRetry_ContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // 立即取消

	calls := 0
	err := DoWithRetry(ctx, RetryConfig{
		MaxAttempts: 5,
		BaseBackoff: 1 * time.Second,
		Jitter:      0,
	}, func(ctx context.Context) (bool, error) {
		calls++
		return true, errors.New("x")
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
	if calls != 1 {
		t.Fatalf("expected 1 call (cancel before retry), got %d", calls)
	}
}
