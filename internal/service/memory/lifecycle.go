// lifecycle.go：记忆库的**生命周期维护**——判据与动作来自底座 agent-memory/lifecycle。
//
// 此前记忆库只进不出：每次评审捕获一条原始事件、若干候选、若干事实卡，
// 但从没有任何一条被回收、被合并、被退役。跑几个月后：
//   - 原始事件表无限增长（每次评审一条，永不删除）
//   - 同一条评审结论被反复记下时，事实面上堆着多张同签名卡
//   - 证据早已过期的断言仍留在召回结果里，以「历史经验」的身份误导模型
//
// 记忆是**长期资产**，长期资产必须能被维护。本文件把底座的三件事接出来：
//   - GC：按龄回收原始事件，并顺带退役「证据全没了」的事实
//     （无法回溯的断言既不能审计也不能推翻）
//   - Consolidate：合并同签名事实（保留最早那张，其余软退役并迁移证据）
//   - Retention：退役「证据全部来自运行期错误」的事实
//
// 底座只提供**机制**（怎么判、怎么动），宿主提供**策略**（多久回收一次、
// 保留多久）。策略参数走配置，业务配置不写死在这里。
package memory

import (
	"context"
	"fmt"
	"time"

	"github.com/yi-nology/agent-memory/lifecycle"
	"github.com/yi-nology/agent-memory/store"
)

// MaintenanceResult 是一次维护动作的结果统计。
type MaintenanceResult struct {
	// RawDeleted 回收的原始事件数。
	RawDeleted int
	// AtomsRetired 因证据全没而顺带退役的事实数。
	AtomsRetired int
	// Merged 合并掉的事实数（被继任者取代的那些）。
	Merged int
	// EvidenceMoved 合并时迁移的证据条数。
	EvidenceMoved int
	// Retired 保留期退役的事实数。
	Retired int
}

// Empty 报告本次维护是否什么都没做。
func (r MaintenanceResult) Empty() bool {
	return r.RawDeleted == 0 && r.AtomsRetired == 0 && r.Merged == 0 && r.Retired == 0
}

// Maintain 对某个命名空间执行一轮维护。
//
// rawRetention 是原始事件的保留时长：早于「现在 - rawRetention」的事件被回收。
// 传 0 表示不回收原始事件（只做合并与退役）。
//
// 顺序刻意是 GC → Consolidate → Retention：
//   - 先回收，让「证据已过期」的事实在合并前就完成退役判定
//   - 再合并，把同签名的卡归一
//   - 最后按保留期退役剩余的运行期错误事实
//
// 顺序反了会出现「先合并出一张卡、随后 GC 把它的证据删光」，
// 结果是一张既没被合并掉也没被退役的悬空卡。
func (s *Service) Maintain(ctx context.Context, ns store.Namespace, rawRetention time.Duration) (MaintenanceResult, error) {
	var res MaintenanceResult
	st := s.app.Store()

	// ① 回收原始事件（并顺带退役证据全没的事实）。
	if rawRetention > 0 {
		before := time.Now().UTC().Add(-rawRetention)
		gcRes, err := lifecycle.GC(ctx, st, ns, before)
		if err != nil {
			return res, fmt.Errorf("回收记忆原始事件: %w", err)
		}
		res.RawDeleted = gcRes.RawDeleted
		res.AtomsRetired = gcRes.AtomsRetired
	}

	// ② 合并同签名事实。
	consRes, err := lifecycle.Consolidate(ctx, st, ns)
	if err != nil {
		return res, fmt.Errorf("合并同签名记忆事实: %w", err)
	}
	res.Merged = consRes.Merged
	res.EvidenceMoved = consRes.EvidenceMoved

	// ③ 保留期：退役证据全部来自运行期错误的事实。
	retRes, err := lifecycle.Retention(ctx, st, ns)
	if err != nil {
		return res, fmt.Errorf("执行记忆保留期: %w", err)
	}
	res.Retired = retRes.Retired

	return res, nil
}

// MaintainAll 对所有已知命名空间执行一轮维护。
//
// 命名空间从**宿主自己的登记表**取（见 rememberNamespace），不从底座反查——
// 底座的存储接口全部按命名空间过滤，没有跨命名空间列举的能力
// （`ListEntities` 的 SQL 是 `WHERE namespace = ?`）。
// 反查只能拿到某个已知 ns 下的实体，那不是「有哪些 ns」。
//
// 登记表是**进程内**的：它记的是「本实例碰过哪些 ns」。
// 后果是多副本部署时每个副本只维护自己碰过的 ns——
// 这不影响正确性（每个 ns 的维护动作幂等，且 GC/合并对同一 ns 并发执行
// 结果一致），只是各副本维护的覆盖面不同。
// 真正的「全域维护」由运维侧或一次性任务触发。
func (s *Service) MaintainAll(ctx context.Context, rawRetention time.Duration) (MaintenanceResult, error) {
	var total MaintenanceResult

	for _, ns := range s.knownNamespaces() {
		res, err := s.Maintain(ctx, ns, rawRetention)
		if err != nil {
			// 单个 ns 维护失败不应中断其余 ns：一次失败多半是存储抖动，
			// 而漏掉一个 ns 的维护只是那个 ns 的数据多留一阵。
			return total, fmt.Errorf("维护命名空间 %s: %w", ns, err)
		}
		total.RawDeleted += res.RawDeleted
		total.AtomsRetired += res.AtomsRetired
		total.Merged += res.Merged
		total.EvidenceMoved += res.EvidenceMoved
		total.Retired += res.Retired
	}

	return total, nil
}

// knownNamespaces 返回本实例已登记的命名空间（拷贝，调用方不得改写本包状态）。
func (s *Service) knownNamespaces() []store.Namespace {
	s.nsMu.RLock()
	defer s.nsMu.RUnlock()

	out := make([]store.Namespace, 0, len(s.namespaces))
	for ns := range s.namespaces {
		out = append(out, ns)
	}
	return out
}

// rememberNamespace 登记一个命名空间，供后台维护扫描。
//
// 登记必须在**捕获与召回两条路径上都做**：只登记捕获路径的话，
// 「只被召回过、从没捕获过」的仓库不会进维护范围——
// 而那正是召回路径会长期读取的 ns，其数据同样会无限增长。
func (s *Service) rememberNamespace(ns store.Namespace) {
	if ns == "" {
		return
	}
	s.nsMu.Lock()
	defer s.nsMu.Unlock()
	if s.namespaces == nil {
		s.namespaces = make(map[store.Namespace]struct{})
	}
	s.namespaces[ns] = struct{}{}
}
