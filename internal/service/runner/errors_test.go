package runner

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestClassifyExitError(t *testing.T) {
	timeout := 10 * time.Minute

	t.Run("deadline exceeded -> TimeoutError", func(t *testing.T) {
		ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
		defer cancel()
		err := classifyExitError(ctx, errors.New("signal: killed"), timeout, "partial stderr")
		if !IsTimeout(err) {
			t.Fatalf("expected TimeoutError, got %T: %v", err, err)
		}
		te, ok := err.(*TimeoutError)
		if !ok {
			t.Fatalf("not *TimeoutError: %T", err)
		}
		if te.Timeout != timeout {
			t.Errorf("timeout mismatch: got %s want %s", te.Timeout, timeout)
		}
		if IsExit(err) || IsBinaryMissing(err) {
			t.Error("TimeoutError should not also match Exit/BinaryMissing")
		}
	})

	t.Run("non-deadline exit -> ExitError preserves stderr", func(t *testing.T) {
		inner := errors.New("exit status 1")
		err := classifyExitError(context.Background(), inner, timeout, "dial tcp: lookup xiaomimimo.com: no such host")
		if !IsExit(err) {
			t.Fatalf("expected ExitError, got %T", err)
		}
		ee := err.(*ExitError)
		if ee.Stderr == "" {
			t.Error("stderr should be preserved for 等排障")
		}
		if !errors.Is(err, inner) {
			t.Error("Unwrap should expose inner error")
		}
		if IsTimeout(err) {
			t.Error("ExitError should not match IsTimeout")
		}
	})
}

func TestExitErrorMessage(t *testing.T) {
	if got := (&ExitError{Stderr: "  dns fail  "}).Error(); got != "ocr 评审失败: dns fail" {
		t.Errorf("stderr should be trimmed in message, got %q", got)
	}
	if got := (&ExitError{Err: errors.New("boom")}).Error(); got != "ocr 评审失败: boom" {
		t.Errorf("fallback to err message: got %q", got)
	}
	if got := (&ExitError{}).Error(); got != "ocr 评审失败" {
		t.Errorf("empty exit error message: got %q", got)
	}
}

func TestBinaryMissingError(t *testing.T) {
	inner := errors.New("exec: \"ocr\": executable file not found in $PATH")
	err := &BinaryMissingError{Binary: "ocr", Err: inner}
	if !errors.Is(err, inner) {
		t.Error("Unwrap should expose inner")
	}
	if !IsBinaryMissing(err) {
		t.Error("IsBinaryMissing should be true")
	}
	if got := err.Error(); got == "" {
		t.Error("message should not be empty")
	}
}
