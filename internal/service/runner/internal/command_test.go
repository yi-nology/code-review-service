package internal

import (
	"bytes"
	"context"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestRun_CapturesStdoutAndStderr(t *testing.T) {
	res, err := Run(context.Background(), CommandSpec{
		Binary: "sh",
		Args:   []string{"-c", "echo hello-stdout; echo hello-stderr 1>&2"},
	})
	if err != nil {
		t.Fatalf("expected nil err, got %v", err)
	}
	if !bytes.Contains(res.Stdout, []byte("hello-stdout")) {
		t.Fatalf("expected stdout to contain hello-stdout, got %q", res.Stdout)
	}
	if !bytes.Contains(res.Stderr, []byte("hello-stderr")) {
		t.Fatalf("expected stderr to contain hello-stderr, got %q", res.Stderr)
	}
	if res.ExitCode != 0 {
		t.Fatalf("expected exit 0, got %d", res.ExitCode)
	}
}

func TestRun_RespectsTimeout(t *testing.T) {
	start := time.Now()
	res, err := Run(context.Background(), CommandSpec{
		Binary:  "sleep",
		Args:    []string{"5"},
		Timeout: 100 * time.Millisecond,
		KillGrace: 200 * time.Millisecond,
	})
	elapsed := time.Since(start)
	if err == nil {
		t.Fatalf("expected non-nil err on timeout")
	}
	if !res.TimedOut {
		t.Fatalf("expected TimedOut=true")
	}
	if elapsed > 1*time.Second {
		t.Fatalf("expected killed within 1s, took %v", elapsed)
	}
}

func TestRun_OnStderrCallback(t *testing.T) {
	var lines []string
	var mu sync.Mutex
	res, err := Run(context.Background(), CommandSpec{
		Binary: "sh",
		Args:   []string{"-c", "echo line1 1>&2; echo line2 1>&2; echo line3 1>&2"},
		OnStderr: func(l string) {
			mu.Lock()
			defer mu.Unlock()
			lines = append(lines, l)
		},
	})
	if err != nil {
		t.Fatalf("expected nil err, got %v", err)
	}
	if !res.TimedOut == false {
		// 防止 lint 误报
		_ = res.TimedOut
	}
	mu.Lock()
	defer mu.Unlock()
	if len(lines) != 3 {
		t.Fatalf("expected 3 stderr lines, got %d: %v", len(lines), lines)
	}
	if !strings.Contains(lines[0], "line1") {
		t.Fatalf("expected first line to contain 'line1', got %q", lines[0])
	}
}

func TestRun_EmptyBinary(t *testing.T) {
	_, err := Run(context.Background(), CommandSpec{Binary: ""})
	if err == nil {
		t.Fatalf("expected error for empty binary")
	}
}

func TestRun_ExitCodeOnFailure(t *testing.T) {
	res, err := Run(context.Background(), CommandSpec{
		Binary: "sh",
		Args:   []string{"-c", "exit 7"},
	})
	if err == nil {
		t.Fatalf("expected non-nil err for non-zero exit")
	}
	if res.ExitCode != 7 {
		t.Fatalf("expected exit code 7, got %d", res.ExitCode)
	}
}
