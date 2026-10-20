package workflow

import (
	"context"
	"time"

	"github.com/oekit/logx"
	"github.com/yi-nology/code-review-service/internal/service/agent"
	"github.com/yi-nology/code-review-service/internal/service/discussion"
	"github.com/yi-nology/code-review-service/internal/service/progress"
	"github.com/yi-nology/code-review-service/internal/service/requirements"
)

// ReviewSession 在各步骤间共享的会话数据
type ReviewSession struct {
	ReviewID  string
	SessionID string
	Round     int
	Request   ReviewRequest
	StartTime time.Time

	// 中间结果
	Workspace        agent.Workspace
	ParsedReqs       *requirements.Requirements
	AggregatedResult *agent.AggregatedResult
	DiscussionResult *discussion.DiscussionResult
	Requirements     *requirements.CoverageAssessment
	TechLeadAnalysis *TechLeadAnalysis
	FinalReport      *FinalReport

	// 进度追踪
	Tracker *progress.ProgressTracker

	// Kernel 是底座适配层：阶段推进、进度快照与终态独占提交都走它。
	// 阶段状态迁移的合法性由 agent-harness/statemachine 裁决，
	// 宿主不另立状态表。
	Kernel *phaseController
}

// enterPhase 推进到某个阶段。
//
// 失败只记日志不中断评审——阶段推进失败属于「进度没跟上」，
// 不该把一次本来能跑完的评审判死（与重构前的行为一致）。
func (rs *ReviewSession) enterPhase(ctx context.Context, phase, message string) {
	if rs.Kernel == nil {
		rs.Tracker.UpdatePhase(progress.Phase(phase), "active", 0, message)
		return
	}
	if err := rs.Kernel.begin(ctx, phase, message); err != nil {
		logx.Errorf("advance review phase %s failed: %v", phase, err)
	}
}

// completePhase 收口某个阶段（成功终态）。
func (rs *ReviewSession) completePhase(phase, message string) {
	if rs.Kernel == nil {
		rs.Tracker.UpdatePhase(progress.Phase(phase), "completed", 100, message)
		return
	}
	if err := rs.Kernel.end(phase, phaseSucceeded, message); err != nil {
		logx.Errorf("complete review phase %s failed: %v", phase, err)
	}
}

// failCurrentPhase 把当前阶段收成失败终态（步骤失败时调用）。
//
// 走 kernel 而不是自己改状态：阶段状态的合法转移表由底座 statemachine
// 裁决，宿主不另立一份（否则两份表会漂移）。
func (rs *ReviewSession) failCurrentPhase(message string) {
	if rs.Kernel == nil {
		return
	}
	if err := rs.Kernel.failCurrent(message); err != nil {
		logx.Errorf("fail review phase failed: %v", err)
	}
}

// ReviewRequest contains the review request parameters
type ReviewRequest struct {
	ReviewID     string
	Provider     string
	Repo         string
	BaseSHA      string
	HeadSHA      string
	Diff         string
	Dir          string
	Requirements []string
	Round        int
	// Language 控制 TechLeadAnalysis 等 LLM 生成内容的回复语言。
	Language string
	// RulePath 是合并后的规则文件路径（项目+规则包+全局规则），透传给每个 agent 的 --rule。
	// 修复：workflow 路径此前丢失该字段，导致多 agent 模式下全局/项目规则不生效。
	RulePath string
	// EphemeralReviewers 是本次评审临时加入的 reviewer（阶段 5），不入库。
	EphemeralReviewers []agent.EphemeralSpec
	// MemoryContext 是召回的跨会话长期记忆（底座 agent-memory），
	// 透传给每个 agent，拼进 OCR 的 --background。
	MemoryContext string
}

// ReviewResult contains the final review result
type ReviewResult struct {
	SessionID        string
	Round            int
	AgentResults     []agent.AgentResult
	Discussion       *discussion.DiscussionResult
	Requirements     *requirements.CoverageAssessment
	TechLeadAnalysis *TechLeadAnalysis
	FinalReport      FinalReport
	Duration         time.Duration
}

// FinalReport contains the final review report
type FinalReport struct {
	Verdict             string
	Summary             string
	Blockers            []agent.Finding
	ShouldFix           []agent.Finding
	Suggestions         []agent.Finding
	Score               int
	GatePassed          bool
	GateReason          string
	ClarifyingQuestions []ClarifyingQuestion `json:"clarifying_questions,omitempty"`
}

// ClarifyingQuestion 是需向利益相关者澄清的问题（阶段 4）。
type ClarifyingQuestion struct {
	Category string `json:"category"`
	Question string `json:"question"`
	Context  string `json:"context"`
}

// TechLeadAnalysis 是 Tech Lead（Phase 3）的 LLM 分析产物（B2）。
type TechLeadAnalysis struct {
	Summary         string   `json:"summary"`
	Risks           []string `json:"risks"`
	Recommendations []string `json:"recommendations"`
}
