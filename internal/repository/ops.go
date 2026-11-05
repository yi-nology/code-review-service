package repository

import (
	"context"
	"time"
)

// LeaseRepository 多副本部署下的周期任务租约数据面。
//
// 多副本后，ArgusPoller sweep / memory maintainer / worker 启动收敛在每个副本
// 都会跑；其中 recoverReport 会写 findings/通知/commit status，双跑即双写。
// 租约 = TTL 内单持有者，抢不到的副本跳过本 tick。
type LeaseRepository interface {
	// TryAcquire 尝试抢占 name 的租约（TTL 内唯一持有者）。
	// 返回 true = 抢到，调用方执行任务；false = 他方持有，跳过。
	// TTL 过期自动可抢（故障副本的租约不会永久卡死任务）。
	TryAcquire(ctx context.Context, name string, ttl time.Duration) bool
}

// MemoryNamespaceRepository 记忆库命名空间注册表（宿主侧共享）。
//
// 底座 agent-memory 存储接口按命名空间过滤、无跨命名空间列举能力，「有哪些 ns
// 要维护」由宿主记。落库后多副本共享、重启不丢（此前是进程内存 map：多副本
// 各记各的，单副本重启也丢）。
type MemoryNamespaceRepository interface {
	// TouchNS 记录/刷新一个命名空间（幂等 upsert）。
	TouchNS(ctx context.Context, ns string) error
	// ListNS 返回全量命名空间（全副本共享视图）。
	ListNS(ctx context.Context) ([]string, error)
}
