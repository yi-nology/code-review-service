package progress

import (
	"context"
	"time"

	dprogress "github.com/yi-nology/code-review-service/internal/domain/progress"
)

// 本包是进度的**机制层**（tracker / manager / websocket 桥）；词汇与端口在
// internal/domain/progress。
//
// 下面这组别名让既有调用点（progress.Phase / progress.ProgressReporter …）
// 保持不变，同时让「只要词汇」的消费方（如 internal/ocr）可以只依赖 domain
// 包、不必拖上整个服务包。

// Phase 是评审流程的阶段名（领域词汇）。
type Phase = dprogress.Phase

// 评审阶段名。
const (
	PhaseContext       = dprogress.PhaseContext
	PhaseChangeContext = dprogress.PhaseChangeContext
	PhaseAnalysis      = dprogress.PhaseAnalysis
	PhaseReviews       = dprogress.PhaseReviews
	PhaseAggregation   = dprogress.PhaseAggregation
	PhaseDiscussion    = dprogress.PhaseDiscussion
	PhaseSynthesis     = dprogress.PhaseSynthesis
	PhaseComplete      = dprogress.PhaseComplete
)

// ProgressReporter 是进度上报端口。
type ProgressReporter = dprogress.ProgressReporter

// AgentStatus 是单个 Agent 的状态快照。
type AgentStatus = dprogress.AgentStatus

// PhaseInfo 是单个阶段的快照。
type PhaseInfo = dprogress.PhaseInfo

// WithProgressReporter 把上报端口放进 ctx。
func WithProgressReporter(ctx context.Context, reporter ProgressReporter) context.Context {
	return dprogress.WithProgressReporter(ctx, reporter)
}

// ProgressReporterFromContext 从 ctx 取上报端口；没有则返回 nil。
func ProgressReporterFromContext(ctx context.Context) ProgressReporter {
	return dprogress.ProgressReporterFromContext(ctx)
}

// eventType 是进度事件的类型（仅本包的机制层使用，不对外暴露）。
type eventType string

const (
	eventPhaseStart    eventType = "phase_start"
	eventPhaseProgress eventType = "phase_progress"
	eventPhaseComplete eventType = "phase_complete"
	eventAgentStatus   eventType = "agent_status"
	eventError         eventType = "error"
	eventComplete      eventType = "complete"
)

// progressEvent 是进度事件载荷（仅本包的机制层使用，不对外暴露）。
type progressEvent struct {
	Type        eventType              `json:"type"`
	Phase       Phase                  `json:"phase"`
	Status      string                 `json:"status"`
	Progress    float64                `json:"progress"`
	Message     string                 `json:"message"`
	AgentStatus map[string]AgentStatus `json:"agent_status,omitempty"`
	Error       string                 `json:"error,omitempty"`
	Timestamp   time.Time              `json:"timestamp"`
}
