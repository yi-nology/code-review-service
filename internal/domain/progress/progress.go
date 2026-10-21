// Package progress 定义评审进度的**词汇与端口**。
//
// 为什么独立于 internal/service/progress：进度事件是评审的**领域词汇**
// （阶段名、上报动作、状态快照），不是某个服务的实现细节。OCR 子进程要上报
// 阶段进度、service 层要读进度快照、handler 要把它转成 API 结构——三方都
// 只需要这份词汇。
//
// 此前词汇与机制同在 internal/service/progress，于是 internal/ocr 为了
// 引用一个阶段名常量与一个上报端口，被迫依赖整个服务包（架构门禁
// ocr-no-service-dep 记的就是这条边）。
//
// 拆成两层后：
//   - 本包（domain）：词汇 + 端口 + ctx 管道，零内部依赖、零 infra 依赖；
//   - internal/service/progress：机制（tracker / manager / websocket 桥），
//     依赖本包并实现本包的端口。
package progress

import (
	"context"
	"time"

	dsession "github.com/yi-nology/code-review-service/internal/domain/session"
)

// Phase 是评审流程的阶段名。
//
// 取值直接引用领域层的阶段枚举（单一事实源）——此前本包另持一份字面量副本，
// 与 internal/domain/session 并存，改一处忘另一处就会漂移。
type Phase string

// 评审阶段名（取值来自 internal/domain/session）。
const (
	PhaseContext       Phase = dsession.PhaseContext
	PhaseChangeContext Phase = dsession.PhaseChangeContext
	PhaseAnalysis      Phase = dsession.PhaseAnalysis
	PhaseReviews       Phase = dsession.PhaseReviews
	PhaseAggregation   Phase = dsession.PhaseAggregation
	PhaseDiscussion    Phase = dsession.PhaseDiscussion
	PhaseSynthesis     Phase = dsession.PhaseSynthesis
	PhaseComplete      Phase = dsession.PhaseComplete
)

// ProgressReporter 是进度上报端口（消费方定义）。
//
// 实现方是 internal/service/progress 的 tracker；但任何需要「上报进度」的
// 消费方（OCR 子进程、评审服务、扫描编排）都只该依赖这个端口。
type ProgressReporter interface {
	ReportProgress(message string, progress float64)
	ReportFileProgress(filename string, current, total int)
	ReportAgentStatus(agentName, status, message string)
	UpdatePhase(phase Phase, status string, progress float64, message string)
}

// progressReporterKey 是 ctx 里存放上报端口的键。
type progressReporterKey struct{}

// WithProgressReporter 把上报端口放进 ctx。
func WithProgressReporter(ctx context.Context, reporter ProgressReporter) context.Context {
	return context.WithValue(ctx, progressReporterKey{}, reporter)
}

// ProgressReporterFromContext 从 ctx 取上报端口；没有则返回 nil。
func ProgressReporterFromContext(ctx context.Context) ProgressReporter {
	if reporter, ok := ctx.Value(progressReporterKey{}).(ProgressReporter); ok {
		return reporter
	}
	return nil
}

// AgentStatus 是单个 Agent 的状态快照。
type AgentStatus struct {
	Name     string  `json:"name"`
	Status   string  `json:"status"`
	Progress float64 `json:"progress"`
	Message  string  `json:"message"`
	Duration int64   `json:"duration_ms,omitempty"`
	Error    string  `json:"error,omitempty"`
}

// PhaseInfo 是单个阶段的快照。
type PhaseInfo struct {
	Name        string    `json:"name"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Status      string    `json:"status"`
	Progress    float64   `json:"progress"`
	StartTime   time.Time `json:"start_time,omitempty"`
	EndTime     time.Time `json:"end_time,omitempty"`
	Duration    int64     `json:"duration_ms,omitempty"`
}
