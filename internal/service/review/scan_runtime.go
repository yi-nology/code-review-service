package review

import (
	"context"
	"sync"

	"github.com/oekit/concurrency/async"
	"github.com/oekit/concurrency/syncx"
	"github.com/oekit/logx"
)

// scanRuntime 聚合 scan 子系统所需的进程内生命周期设施：并发闸门、
// in-flight 计数、可取消扫描注册表。由 Service 构造期创建并持有，
// ScanOrchestrator 通过它使用共享设施，而非直接摸 Service 的全部字段。
type scanRuntime struct {
	// gate 在飞闸门：cap<=0 表示不限制。上游 syncx.Gate 承担「阻塞取名额 +
	// ctx 取消退出」与「幂等归还名额」，替掉此前自维护的 chan struct{} 信号量
	// （归还名额与取名额分离，重复归还会把名额多还一个）。
	gate        *syncx.Gate
	inFlight    sync.WaitGroup
	scanCancels sync.Map // reviewID → context.CancelFunc
}

// acquire 取一个在飞名额（阻塞直到有可用容量或 ctx 取消）。
// 返回的 release 归还名额（幂等，可重复调用）；第二个返回值 false 表示 ctx 已取消。
func (r *scanRuntime) acquire(ctx context.Context) (func(), bool) {
	release, err := r.gate.Acquire(ctx)
	if err != nil {
		return nil, false
	}
	return release, true
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

		release, ok := r.acquire(ctx)
		if !ok {
			if id != "" {
				logx.Warnf("scan %s cancelled while waiting for concurrency slot", id)
			}
			return
		}
		defer release()

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
