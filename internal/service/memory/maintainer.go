// maintainer.go：记忆库后台维护循环。
//
// 记忆是长期资产——资产不维护就会一直涨：原始事件每次评审加一条，
// 事实卡每次捕获加若干。跑上几个月，库里的内容大半是重复与过期，
// 而召回仍会把它们一起塞进 prompt（预算有限，等于挤掉了真正有用的记忆）。
//
// 维护动作与判据来自底座 agent-memory/lifecycle（见 lifecycle.go），
// 本文件只负责**何时做**（周期）与**做多久**（超时）。
//
// 维护是后台副产物：失败只记日志，不影响评审；
// 评审链路不依赖维护跑过（记忆的捕获与召回都不读维护状态）。
package memory

import (
	"context"
	"time"

	"github.com/oekit/concurrency/async"
	"github.com/oekit/logx"
)

const (
	// defaultMaintenanceInterval 维护周期缺省值。
	//
	// 6 小时是权衡：更频繁对存储压力大（每次要扫全部事实卡），
	// 更稀疏则回收滞后、库涨得快。记忆的写入是「每次评审几条」，
	// 6 小时一次足够把增长压住。
	defaultMaintenanceInterval = 6 * time.Hour

	// defaultRawRetention 原始事件保留期缺省值。
	//
	// 30 天：足够覆盖「一条事实被质疑时回头查证」的窗口，
	// 又不至于让原始事件无限堆积。事实卡本身不随原始事件删除而消失，
	// 只是「证据全没了」的事实会被 GC 顺带退役。
	defaultRawRetention = 30 * 24 * time.Hour

	// maintenanceTimeout 单轮维护的超时上限。
	//
	// 维护扫的是全部事实卡，库大了会慢；不设上限则一次卡住的维护
	// 会占住循环 forever，后续周期全部错过。
	maintenanceTimeout = 30 * time.Minute
)

// Maintainer 是记忆库的后台维护器。
type Maintainer struct {
	svc          *Service
	interval     time.Duration
	rawRetention time.Duration
	stopCh       chan struct{}
	doneCh       chan struct{}
}

// MaintainerOption 是维护器配置项。
type MaintainerOption func(*Maintainer)

// WithInterval 设置维护周期。
func WithInterval(d time.Duration) MaintainerOption {
	return func(m *Maintainer) {
		if d > 0 {
			m.interval = d
		}
	}
}

// WithRawRetention 设置原始事件保留期。
func WithRawRetention(d time.Duration) MaintainerOption {
	return func(m *Maintainer) {
		if d > 0 {
			m.rawRetention = d
		}
	}
}

// NewMaintainer 构造记忆库维护器。
//
// svc 为 nil 时返回 nil——记忆未启用时没有维护可言，
// 调用方按 nil 跳过（与记忆服务本身的降级口径一致）。
func NewMaintainer(svc *Service, opts ...MaintainerOption) *Maintainer {
	m := &Maintainer{
		svc:          svc,
		interval:     defaultMaintenanceInterval,
		rawRetention: defaultRawRetention,
		stopCh:       make(chan struct{}),
		doneCh:       make(chan struct{}),
	}
	for _, o := range opts {
		if o != nil {
			o(m)
		}
	}
	return m
}

// Start 启动维护循环。
//
// 首轮**不立即执行**：服务刚起时记忆库里往往还没有内容，
// 立刻扫一遍是白扫。等第一个周期再动。
func (m *Maintainer) Start() {
	if m == nil || m.svc == nil {
		return
	}
	logx.Infof("memory maintainer started: interval=%v raw_retention=%v", m.interval, m.rawRetention)

	ticker := time.NewTicker(m.interval)
	defer ticker.Stop()

	for {
		select {
		case <-m.stopCh:
			close(m.doneCh)
			return
		case <-ticker.C:
			m.runOnce()
		}
	}
}

// runOnce 执行一轮维护。
func (m *Maintainer) runOnce() {
	ctx, cancel := context.WithTimeout(context.Background(), maintenanceTimeout)
	defer cancel()

	res, err := m.svc.MaintainAll(ctx, m.rawRetention)
	if err != nil {
		// 维护失败只记日志：它是副产物，不该让服务看起来出了问题。
		logx.Warnf("memory maintenance failed: %v", err)
		return
	}
	if res.Empty() {
		return
	}
	logx.Infof("memory maintenance done: raw_deleted=%d atoms_retired=%d merged=%d evidence_moved=%d retired=%d",
		res.RawDeleted, res.AtomsRetired, res.Merged, res.EvidenceMoved, res.Retired)
}

// Stop 停止维护循环并等待退出。
//
// 等待退出是必须的：维护动作在写存储，不等它退出就关库，
// 会让一轮维护撞上关闭中的连接。
func (m *Maintainer) Stop(ctx context.Context) error {
	if m == nil || m.svc == nil {
		return nil
	}
	close(m.stopCh)
	select {
	case <-m.doneCh:
		logx.Info("memory maintainer stopped")
		return nil
	case <-ctx.Done():
		logx.Warnf("memory maintainer stop timed out: %v", ctx.Err())
		return ctx.Err()
	}
}

// StartAsync 异步启动维护循环（供 fx OnStart 用，避免阻塞启动）。
func (m *Maintainer) StartAsync() {
	if m == nil || m.svc == nil {
		return
	}
	async.GoSafe(func() {
		m.Start()
	})
}
