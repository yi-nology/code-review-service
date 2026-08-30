package internal

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"
)

func TestRun_CapturesStdout(t *testing.T) {
	res, err := Run(context.Background(), CommandSpec{
		Binary: "sh",
		Args:   []string{"-c", "echo hello"},
	})
	if err != nil {
		t.Fatalf("Run err: %v", err)
	}
	if !bytes.Contains(res.Stdout, []byte("hello")) {
		t.Fatalf("expected stdout=hello, got %q", res.Stdout)
	}
	if res.ExitCode != 0 {
		t.Errorf("expected exit 0, got %d", res.ExitCode)
	}
}

func TestRun_RespectsTimeout(t *testing.T) {
	start := time.Now()
	res, err := Run(context.Background(), CommandSpec{
		Binary:  "sleep",
		Args:    []string{"5"},
		Timeout: 100 * time.Millisecond,
	})
	if err == nil {
		t.Fatalf("expected err on timeout")
	}
	if !res.TimedOut {
		t.Errorf("expected TimedOut=true")
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Errorf("expected kill within 2s, took %v", elapsed)
	}
}

func TestRun_SendsSIGTERMBeforeKill(t *testing.T) {
	// 子进程捕获 SIGTERM 后优雅退出：三段式第一段必须把 SIGTERM 送到进程组。
	res, err := Run(context.Background(), CommandSpec{
		Binary: "sh",
		Args:   []string{"-c", "trap 'echo caught-term; exit 0' TERM; echo started; sleep 5"},
		// stdout/stderr 是管道缓冲，先 flush started 再触发超时。
		Timeout:   300 * time.Millisecond,
		KillGrace: time.Second,
	})
	_ = err // 优雅退出时 Wait 可能返回 nil，也可能带 terminated 状态，不作断言
	if !bytes.Contains(res.Stdout, []byte("caught-term")) {
		t.Fatalf("expected SIGTERM to be delivered (stdout marker), got %q", res.Stdout)
	}
}

func TestRun_KillGraceThenSIGKILL(t *testing.T) {
	// 子进程忽略 SIGTERM：必须在 Timeout+KillGrace+余量 内被 SIGKILL。
	start := time.Now()
	res, err := Run(context.Background(), CommandSpec{
		Binary:    "sh",
		Args:      []string{"-c", "trap '' TERM; sleep 5"},
		Timeout:   100 * time.Millisecond,
		KillGrace: 300 * time.Millisecond,
	})
	if err == nil {
		t.Fatalf("expected err when child ignores SIGTERM")
	}
	if !res.TimedOut {
		t.Errorf("expected TimedOut=true")
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Errorf("expected SIGKILL within ~400ms after grace, took %v", elapsed)
	}
}

func TestRun_EmptyBinary(t *testing.T) {
	if _, err := Run(context.Background(), CommandSpec{}); err == nil {
		t.Errorf("expected err for empty binary")
	}
}

func TestRun_StderrCallback(t *testing.T) {
	var lines []string
	_, _ = Run(context.Background(), CommandSpec{
		Binary: "sh",
		Args:   []string{"-c", "echo err1 1>&2; echo err2 1>&2"},
		OnStderr: func(l string) {
			lines = append(lines, l)
		},
	})
	if len(lines) != 2 {
		t.Fatalf("expected 2 stderr lines, got %d: %v", len(lines), lines)
	}
	if !strings.Contains(lines[0], "err1") {
		t.Errorf("expected err1 first, got %q", lines[0])
	}
}

func TestRun_NonZeroExitCode(t *testing.T) {
	res, err := Run(context.Background(), CommandSpec{
		Binary: "sh",
		Args:   []string{"-c", "exit 7"},
	})
	if err == nil {
		t.Fatalf("expected err for non-zero exit")
	}
	if res.ExitCode != 7 {
		t.Errorf("expected exit 7, got %d", res.ExitCode)
	}
}
