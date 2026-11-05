package postgres

import (
	"context"
	"testing"
	"time"
)

// TestLeaseTryAcquire 租约语义：TTL 内唯一持有，过期后可再抢。
// 多副本单例化的核心保证（ArgusPoller sweep / memory maintainer / startup backfill）。
func TestLeaseTryAcquire(t *testing.T) {
	if testDB == nil {
		t.Skip("testDB not initialized (docker unavailable)")
	}
	ctx := context.Background()

	r := NewLeaseRepo(testDB)
	const name = "test-lease-basic"

	if !r.TryAcquire(ctx, name, time.Minute) {
		t.Fatal("空表首抢应成功")
	}
	if r.TryAcquire(ctx, name, time.Minute) {
		t.Fatal("TTL 内再抢应失败（单持有者语义）")
	}

	// 模拟过期：直接把 expires_at 拨回过去。
	if err := testDB.Exec("UPDATE job_leases SET expires_at = now() - interval '1 minute' WHERE name = ?", name).Error; err != nil {
		t.Fatalf("拨回 expires_at: %v", err)
	}
	if !r.TryAcquire(ctx, name, time.Minute) {
		t.Fatal("过期后应可再抢（故障副本不卡死任务）")
	}
}

// TestLeaseTryAcquireConcurrent 并发抢租互斥：N 个 goroutine 同 tick 抢同一租约，
// 恰好一个成功（多副本同时启动/同 tick 竞争的真实形态）。
func TestLeaseTryAcquireConcurrent(t *testing.T) {
	if testDB == nil {
		t.Skip("testDB not initialized (docker unavailable)")
	}
	ctx := context.Background()
	r := NewLeaseRepo(testDB)
	const name = "test-lease-concurrent"

	const n = 8
	results := make(chan bool, n)
	for i := 0; i < n; i++ {
		go func() {
			results <- r.TryAcquire(ctx, name, time.Minute)
		}()
	}
	wins := 0
	for i := 0; i < n; i++ {
		if <-results {
			wins++
		}
	}
	if wins != 1 {
		t.Fatalf("N 个并发抢租应恰好一个成功，got %d", wins)
	}
}

// TestMemoryNSTouchList 命名空间注册表：幂等 touch + 全量列举。
func TestMemoryNSTouchList(t *testing.T) {
	if testDB == nil {
		t.Skip("testDB not initialized (docker unavailable)")
	}
	ctx := context.Background()
	r := NewMemoryNSRepo(testDB)

	if err := r.TouchNS(ctx, "review/gitlab/acme/a"); err != nil {
		t.Fatalf("touch: %v", err)
	}
	if err := r.TouchNS(ctx, "review/gitlab/acme/a"); err != nil {
		t.Fatalf("touch again: %v", err)
	}
	if err := r.TouchNS(ctx, "review/github/acme/b"); err != nil {
		t.Fatalf("touch b: %v", err)
	}

	nss, err := r.ListNS(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(nss) != 2 {
		t.Fatalf("touch 幂等后应 2 个 ns，got %d: %v", len(nss), nss)
	}
}
