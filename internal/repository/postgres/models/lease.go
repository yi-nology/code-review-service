package models

import "time"

// Lease 多副本部署下的周期任务租约：TTL 内单持有者。
//
// 背景：多副本后，ArgusPoller sweep、memory maintainer、worker 启动收敛这类
// 周期/启动任务在每个副本都会跑。它们大多幂等，但 ArgusPoller 的 recoverReport
// 会写 findings/通知/commit status——双跑就是双写。租约表让每 tick 只有一个
// 副本抢到执行权，抢不到的副本跳过本 tick（下一 tick 再抢）。
type Lease struct {
	Name      string    `gorm:"column:name;primaryKey;size:128"`
	Holder    string    `gorm:"column:holder;size:256"`
	ExpiresAt time.Time `gorm:"column:expires_at;index"`
	UpdatedAt time.Time `gorm:"column:updated_at"`
}

// MemoryNamespace 记忆库命名空间注册表（宿主侧）。
//
// 底座 agent-memory 的存储接口全部按命名空间过滤，没有跨命名空间列举能力，
// 所以「有哪些 ns 要维护」只能由宿主记。此前是进程内存 map：多副本下各记各的
// （registry 分裂），单副本重启也丢（重启后直到下一次 capture/recall 才重建）。
// 落库后全副本共享、重启不丢。
type MemoryNamespace struct {
	Namespace  string    `gorm:"column:namespace;primaryKey;size:256"`
	LastSeenAt time.Time `gorm:"column:last_seen_at;index"`
	CreatedAt  time.Time `gorm:"column:created_at"`
}

func (MemoryNamespace) TableName() string { return "memory_namespaces" }
func (Lease) TableName() string           { return "job_leases" }
