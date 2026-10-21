package session

// phase.go：评审流程的**阶段顺序表**——全仓唯一一份。
//
// 此前有两份手写副本：internal/service/session/validation.go 的迁移表，
// 以及 internal/domain/progress 的阶段常量。两份必然漂移，
// 且谁改了另一份不会跟着改——这里把顺序表收成单一事实源，
// 进度包改为直接引用本包的常量。
//
// 阶段的**状态机**（created→running→succeeded/failed）不归本包：
// 那是运行时状态而非领域常量，落在 internal/service/workflow/kernel.go，
// 底座是 agent-harness/statemachine（合法转移表由底座裁决，本仓不另立一份）。

// 阶段顺序表：每个阶段之后允许进入的阶段。
//
// PhaseAggregation 有两个后继：讨论开着走 discourse 再到 synthesis，
// 讨论关掉直接到 synthesis。PhaseComplete 是终态，无后继。
var phaseOrder = map[string][]string{
	PhaseContext:       {PhaseChangeContext},
	PhaseChangeContext: {PhaseAnalysis},
	PhaseAnalysis:      {PhaseReviews},
	PhaseReviews:       {PhaseAggregation},
	PhaseAggregation:   {PhaseDiscussion, PhaseSynthesis},
	PhaseDiscussion:    {PhaseSynthesis},
	PhaseSynthesis:     {PhaseComplete},
	PhaseComplete:      {},
}

// phaseSequence 是阶段的规范顺序（用于编排图与快照对齐）。
var phaseSequence = []string{
	PhaseContext,
	PhaseChangeContext,
	PhaseAnalysis,
	PhaseReviews,
	PhaseAggregation,
	PhaseDiscussion,
	PhaseSynthesis,
	PhaseComplete,
}

// phaseInFlightProgress 是阶段「进行中」时写进会话 progress 列的值。
//
// 这些值来自重构前的散落调用（写在各 step 里，语义不一）。收进本表是
// 为了让「阶段推进写什么 progress」只有一个出处，同时保持对外行为不变——
// 随手改成统一值看着更整齐，但那是一次没有必要的可观测变更。
var phaseInFlightProgress = map[string]float64{
	PhaseContext:       0,
	PhaseChangeContext: 50,
	PhaseAnalysis:      50,
	PhaseReviews:       0,
	PhaseAggregation:   50,
	PhaseDiscussion:    0,
	PhaseSynthesis:     0,
	PhaseComplete:      0,
}

// PhaseInFlightProgress 返回阶段进行中时应写入的会话 progress 值。
// 未知阶段返回 0。
func PhaseInFlightProgress(phase string) float64 {
	return phaseInFlightProgress[phase]
}

// CanTransitionPhase 判定阶段顺序上的迁移是否合法。
// from 不在顺序表内一律非法（未知阶段不静默放行）。
func CanTransitionPhase(from, to string) bool {
	allowed, ok := phaseOrder[from]
	if !ok {
		return false
	}
	for _, next := range allowed {
		if next == to {
			return true
		}
	}
	return false
}

// IsKnownPhase 该阶段名是否在评审流程的阶段集合内。
func IsKnownPhase(phase string) bool {
	_, ok := phaseOrder[phase]
	return ok
}

// PhaseOrder 返回阶段的规范顺序（拷贝，调用方不得改写本包状态）。
func PhaseOrder() []string {
	out := make([]string, len(phaseSequence))
	copy(out, phaseSequence)
	return out
}
