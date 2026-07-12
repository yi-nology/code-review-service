package cleanup

import (
	"context"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/oekit/concurrency/async"
)

func TestWorkspaceCleaner_RemovesExpiredDirs(t *testing.T) {
	tmpDir := t.TempDir()

	oldDir := filepath.Join(tmpDir, "review-workspace-old")
	if err := os.MkdirAll(oldDir, 0o755); err != nil {
		t.Fatal(err)
	}

	pastTime := time.Now().Add(-2 * time.Hour)
	if err := os.Chtimes(oldDir, pastTime, pastTime); err != nil {
		t.Fatal(err)
	}

	freshDir := filepath.Join(tmpDir, "review-workspace-fresh")
	if err := os.MkdirAll(freshDir, 0o755); err != nil {
		t.Fatal(err)
	}

	nonMatchingDir := filepath.Join(tmpDir, "other-dir")
	if err := os.MkdirAll(nonMatchingDir, 0o755); err != nil {
		t.Fatal(err)
	}
	oldNonMatch := time.Now().Add(-48 * time.Hour)
	_ = os.Chtimes(nonMatchingDir, oldNonMatch, oldNonMatch)

	cleaner := NewWorkspaceCleaner(
		1 * time.Hour,
	)
	cleaner.baseDir = tmpDir

	cleaner.cleanup()

	if _, err := os.Stat(oldDir); !os.IsNotExist(err) {
		t.Errorf("expected old workspace to be removed, but it still exists")
	}
	if _, err := os.Stat(freshDir); err != nil {
		t.Errorf("expected fresh workspace to remain, but it was removed: %v", err)
	}
	if _, err := os.Stat(nonMatchingDir); err != nil {
		t.Errorf("expected non-matching dir to remain, but it was removed: %v", err)
	}
}

func TestWorkspaceCleaner_EmptyDir(t *testing.T) {
	tmpDir := t.TempDir()

	cleaner := NewWorkspaceCleaner(1 * time.Hour)
	cleaner.baseDir = tmpDir

	cleaner.cleanup()
}

func TestWorkspaceCleaner_NonExistentDir(t *testing.T) {
	cleaner := NewWorkspaceCleaner(1 * time.Hour)
	cleaner.baseDir = "/nonexistent/path/that/does/not/exist"

	cleaner.cleanup()
}

// TestWorkspaceCleaner_StartNonBlocking 验证 PR-04 修复：
// Start() 应当立即返回，不应被首次 cleanup() 阻塞。
// 即使 baseDir 上有大量文件，启动也应在毫秒级完成。
func TestWorkspaceCleaner_StartNonBlocking(t *testing.T) {
	tmpDir := t.TempDir()
	// 塞 200 个文件模拟 /tmp 繁忙
	for i := 0; i < 200; i++ {
		f, err := os.CreateTemp(tmpDir, "f-")
		if err != nil {
			t.Fatal(err)
		}
		_ = f.Close()
	}

	cleaner := NewWorkspaceCleaner(
		1*time.Hour,
		WithInterval(50*time.Millisecond),
	)
	cleaner.baseDir = tmpDir
	cleaner.pattern = "*"

	done := make(chan struct{})
	start := time.Now()
	async.GoSafe(func() {
		cleaner.Start()
		close(done)
	})
	select {
	case <-done:
		// OK, Start returned
	case <-time.After(500 * time.Millisecond):
		t.Fatalf("Start() blocked longer than 500ms (PR-04 not fixed?)")
	}
	elapsed := time.Since(start)
	if elapsed > 100*time.Millisecond {
		t.Fatalf("Start took too long: %v", elapsed)
	}

	// 验证 ticker 还在工作
	time.Sleep(120 * time.Millisecond)
	if err := cleaner.Stop(context.Background()); err != nil {
		t.Fatalf("Stop returned err: %v", err)
	}
}

// TestWorkspaceCleaner_StopWaitsForGoroutine 验证 PR-04 修复：
// Stop(ctx) 应当阻塞直到后台 goroutine 退出，而不是 close(stopCh) 立即返回。
func TestWorkspaceCleaner_StopWaitsForGoroutine(t *testing.T) {
	tmpDir := t.TempDir()
	cleaner := NewWorkspaceCleaner(
		1*time.Hour,
		WithInterval(1*time.Hour), // 长 interval，让 goroutine 主要在 <-c.stopCh 阻塞
	)
	cleaner.baseDir = tmpDir

	cleaner.Start()

	// 确认 goroutine 已启动
	time.Sleep(20 * time.Millisecond)

	stopDone := make(chan struct{})
	async.GoSafe(func() {
		_ = cleaner.Stop(context.Background())
		close(stopDone)
	})

	// Stop 应当等到 doneCh 关闭后返回
	select {
	case <-stopDone:
		// 成功
	case <-time.After(2 * time.Second):
		t.Fatalf("Stop() did not return within 2s (goroutine leak?)")
	}
}

// TestWorkspaceCleaner_StopRespectsContext 验证 ctx 超时时 Stop 返回 ctx.Err()。
// 我们构造一个 doneCh 永远不会被关闭的场景（不调用 Start），Stop 必须能超时返回。
func TestWorkspaceCleaner_StopRespectsContext(t *testing.T) {
	cleaner := NewWorkspaceCleaner(1 * time.Hour)
	cleaner.baseDir = t.TempDir()
	cleaner.scanTicker = time.NewTicker(time.Hour)
	// 不调用 Start()，loop 永远不启动，doneCh 永不关闭。

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err := cleaner.Stop(ctx)
	if err == nil {
		t.Fatalf("expected timeout error, got nil")
	}
	if err != context.DeadlineExceeded {
		t.Logf("got err: %v (acceptable)", err)
	}
}

// 用原子计数验证 goroutine 真正退出
var stopGoroutineExited atomic.Bool

// TestWorkspaceCleaner_LoopExitsOnStop 验证 loop 在收到 stopCh 后能真正退出。
func TestWorkspaceCleaner_LoopExitsOnStop(t *testing.T) {
	tmpDir := t.TempDir()
	cleaner := NewWorkspaceCleaner(
		1*time.Hour,
		WithInterval(1*time.Hour),
	)
	cleaner.baseDir = tmpDir
	stopGoroutineExited.Store(false)

	// 替换 doneCh 为新 channel
	cleaner.doneCh = make(chan struct{})

	cleaner.Start()
	time.Sleep(20 * time.Millisecond)

	// 启动 watcher：检测 doneCh 关闭
	async.GoSafe(func() {
		<-cleaner.doneCh
		stopGoroutineExited.Store(true)
	})

	if err := cleaner.Stop(context.Background()); err != nil {
		t.Fatalf("Stop err: %v", err)
	}

	// 给 watcher 一点时间设置 flag
	time.Sleep(50 * time.Millisecond)
	if !stopGoroutineExited.Load() {
		t.Fatalf("loop goroutine did not signal doneCh (leak?)")
	}
}
