package ocr

import (
	"context"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/yi-nology/code-review-service/internal/service/runner"
)

// TestViewerManager_StartViewer_NoGoroutineLeak 验证 PR-05 修复：
// 连续多次 StartViewer + StopViewer 不应泄漏后台 goroutine。
//
// 旧实现：每次 StartViewer 直接覆盖 vm.done 字段，旧 goroutine 仍会
// close(原 done) 随后退出，但其 cmd/process 关联的对象 + channel
// 在 GC 路径上构成泄漏。
//
// 验证手段：反复启停 20 次后，runtime.NumGoroutine() 不应显著增长。
func TestViewerManager_StartViewer_NoGoroutineLeak(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping goroutine leak test in short mode")
	}

	vm := NewViewerManager(nil, &ConfigResolver{fallback: runner.OpenCodeReviewConfig{}})

	// 预热一次，让 runtime 初始化 worker pool
	runtime.GC()
	time.Sleep(20 * time.Millisecond)
	baseline := runtime.NumGoroutine()

	for i := 0; i < 20; i++ {
		// 用 `true` 代替 ocr viewer 启动后立即退出
		_, _ = vm.StartViewerWithCommand(context.Background(), "test-addr", "true", []string{})
		_, _ = vm.StopViewer(context.Background())
	}

	runtime.GC()
	time.Sleep(50 * time.Millisecond)
	after := runtime.NumGoroutine()

	// 允许少量调度相关的浮动（< 10）
	if after > baseline+10 {
		t.Fatalf("goroutine count grew from %d to %d (leak?)", baseline, after)
	}
}

// TestViewerManager_StartViewer_RejectsWhileRunning 验证同一时刻只允许一个 viewer 进程。
func TestViewerManager_StartViewer_RejectsWhileRunning(t *testing.T) {
	vm := NewViewerManager(nil, &ConfigResolver{fallback: runner.OpenCodeReviewConfig{}})

	// 启动一个会阻塞一段时间的"viewer"
	status, err := vm.StartViewerWithCommand(context.Background(), "test-addr", "sleep", []string{"0.5"})
	if err != nil {
		t.Fatalf("first start err: %v", err)
	}
	if !status.Running {
		t.Fatalf("expected running=true, got %+v", status)
	}

	// 第二次启动应当被拒绝
	status2, err := vm.StartViewerWithCommand(context.Background(), "test-addr-2", "true", []string{})
	if err != nil {
		t.Fatalf("second start err: %v", err)
	}
	if !status2.Running {
		t.Fatalf("expected running=true (rejected path), got %+v", status2)
	}
	if status2.Addr != "test-addr" {
		t.Fatalf("expected to keep test-addr, got %s", status2.Addr)
	}

	// 清理
	stopCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if _, err := vm.StopViewer(stopCtx); err != nil {
		// sleep 0.5 走 SIGTERM 即可，无需 SIGKILL
		if !strings.Contains(err.Error(), "context") {
			t.Logf("stop returned: %v", err)
		}
	}
}
