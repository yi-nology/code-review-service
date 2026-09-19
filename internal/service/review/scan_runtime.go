package review

import (
	"context"
	"sync"
	"time"

	"github.com/yi-nology/code-review-service/internal/service/progress"
	"github.com/yi-nology/code-review-service/internal/service/review/domain"

	"github.com/oekit/concurrency/async"
	"github.com/oekit/logx"
)

// scanRuntime 聚合 scan 子系统所需的进程内生命周期设施：并发信号量、
// in-flight 计数、可取消扫描注册表。由 Service 构造期创建并持有，
// ScanOrchestrator 通过它使用共享设施，而非直接摸 Service 的全部字段。
type scanRuntime struct {
	sem         chan struct{} // nil 表示不限制
	inFlight    sync.WaitGroup
	scanCancels sync.Map // reviewID → context.CancelFunc
}

func (r *scanRuntime) acquire(ctx context.Context) bool {
	if r.sem == nil {
		return true
	}
	select {
	case r.sem <- struct{}{}:
		return true
	case <-ctx.Done():
		return false
	}
}

func (r *scanRuntime) release() {
	if r.sem != nil {
		<-r.sem
	}
}

// registerCancel 注册可取消扫描并派生 scanCtx；返回的 done 必须在扫描结束时调用。
func (r *scanRuntime) registerCancel(ctx context.Context, id string) (context.Context, func()) {
	scanCtx, cancel := context.WithCancel(ctx)
	r.scanCancels.Store(id, cancel)
	return scanCtx, func() {
		cancel()
		r.scanCancels.Delete(id)
	}
}

// runDetached 在后台 goroutine 执行 fn：占用并发槽、登记 in-flight、
// 注册可取消扫描（id 为空时不注册取消项）。ctx 取消时放弃排队直接返回。
func (r *scanRuntime) runDetached(ctx context.Context, id string, fn func(scanCtx context.Context)) {
	r.inFlight.Add(1)
	async.GoSafe(func() {
		defer r.inFlight.Done()

		if !r.acquire(ctx) {
			if id != "" {
				logx.Warnf("scan %s cancelled while waiting for concurrency slot", id)
			}
			return
		}
		defer r.release()

		if id == "" {
			fn(ctx)
			return
		}
		scanCtx, done := r.registerCancel(ctx, id)
		defer done()
		fn(scanCtx)
	})
}

// CancelScan 取消正在执行的 repository scan（若在跑）。
func (r *scanRuntime) CancelScan(id string) {
	if cancel, ok := r.scanCancels.LoadAndDelete(id); ok {
		if c, ok := cancel.(context.CancelFunc); ok {
			c()
		}
	}
}

// Shutdown 等待所有 in-flight scan 完成（供 Service.Shutdown 复用）。
func (r *scanRuntime) WaitIdle(ctx context.Context) error {
	done := make(chan struct{})
	async.GoSafe(func() {
		r.inFlight.Wait()
		close(done)
	})
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// updateScanPhase 是 scan 路径的进度上报便捷封装（reporter 为 nil 时 no-op）。
func updateScanPhase(reporter progress.ProgressReporter, phase progress.Phase, status string, percent float64, message string) {
	if reporter == nil {
		return
	}
	reporter.UpdatePhase(phase, status, percent, message)
}

// scanQueuedResponse 构造 scan 入队的初始 Response。
func scanQueuedResponse(id string) domain.Response {
	return domain.Response{
		ReviewID: id,
		Status:   StatusQueued,
		Summary:  "repository scan queued",
		Gate:     domain.PendingGate(),
		History: []domain.Event{{
			Stage:     stageQueued,
			Status:    statusCompleted,
			Message:   "repository scan queued",
			CreatedAt: time.Now().UTC(),
		}},
	}
}
