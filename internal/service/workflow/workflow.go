package workflow

import (
	"context"
	"fmt"
	"time"

	"github.com/oekit/logx"
	"github.com/oekit/workflow"
	"github.com/yi-nology/code-review-service/internal/ocr"
	"github.com/yi-nology/code-review-service/internal/service/agent"
	"github.com/yi-nology/code-review-service/internal/service/discussion"
	"github.com/yi-nology/code-review-service/internal/service/progress"
)

// ReviewWorkflow orchestrates the multi-agent review process
type ReviewWorkflow struct {
	agentManager    *agent.AgentManager
	sessionService  sessionService
	progressManager *progress.ProgressManager
	discussionOrch  *discussion.Orchestrator
	llmProvider     ocr.Provider
	config          WorkflowConfig
}

// WorkflowConfig contains workflow configuration
type WorkflowConfig struct {
	EnableDiscussion   bool
	EnableRequirements bool
	DiscussionRounds   int
	// ContextWindow 上下文窗口大小（tokens），0=模型默认。
	//
	// 它是 diff 预算的推导依据（见 budget.go）：窗口已知才能算出
	// 「扣掉输出预留后的输入侧上限」与 diff 占其中多少。
	//
	// 刻意做成**函数**而不是值：OCR 设置存 DB 且支持热更，
	// 启动时拍一次快照会让预算用旧窗口算——而窗口是模型端的事实，
	// 变了就该跟着变。故每次推导时现取。
	ContextWindow func() int
}

// resolveContextWindow 取当前上下文窗口大小。
//
// 取不到（未接线 / 设置未就绪）时返回 0，预算走「窗口未知」的保守分支
// （固定上限 + 压力恒为 critical），与重构前的行为一致。
func (c *WorkflowConfig) resolveContextWindow() int {
	if c == nil || c.ContextWindow == nil {
		return 0
	}
	return c.ContextWindow()
}

// NewReviewWorkflow creates a new review workflow
func NewReviewWorkflow(
	agentManager *agent.AgentManager,
	sessionService sessionService,
	progressManager *progress.ProgressManager,
	llmProvider ocr.Provider,
	config WorkflowConfig,
) *ReviewWorkflow {
	return &ReviewWorkflow{
		agentManager:    agentManager,
		sessionService:  sessionService,
		progressManager: progressManager,
		discussionOrch:  discussion.NewOrchestrator(discussion.DefaultDiscussionConfig(), llmProvider),
		llmProvider:     llmProvider,
		config:          config,
	}
}

// RunReview runs the complete review workflow
func (w *ReviewWorkflow) RunReview(ctx context.Context, request ReviewRequest) (*ReviewResult, error) {
	startTime := time.Now()

	// 计算本轮轮次：requeue/重审时应递增（基于已有 session 的最大轮次）。
	// GetNextRound 不可用时回退到调用方指定的 request.Round。
	round := request.Round
	if r, err := w.sessionService.GetNextRound(ctx, request.ReviewID); err == nil && r > 0 {
		round = r
	}

	// Create session
	session, err := w.sessionService.CreateSession(ctx, request.ReviewID, round)
	if err != nil {
		return nil, fmt.Errorf("create session: %w", err)
	}

	// Create progress tracker
	tracker := w.progressManager.CreateTracker(request.ReviewID, session.SessionID, round)
	defer w.progressManager.RemoveTracker(request.ReviewID)

	// Build review session
	rs := &ReviewSession{
		ReviewID:  request.ReviewID,
		SessionID: session.SessionID,
		Round:     round,
		Request:   request,
		StartTime: startTime,
		Tracker:   tracker,
		// Kernel 是底座适配层：阶段推进、进度快照与终态独占提交都走它。
		Kernel: w.newKernel(session.SessionID, session.Phase, tracker),
	}

	// Build and run pipeline
	// v0.19.0 PipeFlow.Run(ctx) error 不再接受 req/sess, 通过 step 字段传入 (rs)
	pipe := w.buildPipeline(rs)
	if err := pipe.Run(ctx); err != nil {
		w.failSession(ctx, session.SessionID, err.Error())
		tracker.SetError(err.Error())
		return nil, err
	}

	// Build result
	result := &ReviewResult{
		SessionID: session.SessionID,
		Round:     round,
		Duration:  time.Since(startTime),
	}

	if rs.AggregatedResult != nil {
		result.AgentResults = rs.AggregatedResult.AgentResults
	}
	result.Discussion = rs.DiscussionResult
	result.Requirements = rs.Requirements
	result.TechLeadAnalysis = rs.TechLeadAnalysis
	if rs.FinalReport != nil {
		result.FinalReport = *rs.FinalReport
	}

	logx.Infof("review completed in %v with %d findings", result.Duration, len(result.AgentResults))
	return result, nil
}

// newKernel 构造阶段推进器。
//
// 会话初始阶段就是首个阶段，推进器据此避免把「同一阶段」写成一次非法迁移。
func (w *ReviewWorkflow) newKernel(
	sessionID string,
	sessionPhase string,
	tracker *progress.ProgressTracker,
) *phaseController {
	return newPhaseController(sessionID, w.sessionService, tracker, sessionPhase)
}

// buildPipeline 构建 review 流程管道
// v0.19.0 Runnable.Run(ctx) error: req/sess 通过 step 字段传入 (rs)
func (w *ReviewWorkflow) buildPipeline(rs *ReviewSession) *workflow.PipeFlow {
	pipe := workflow.NewPipe()

	// Phase 1: Context Discovery
	pipe.Add(&contextDiscoveryStep{rs: rs})

	// Phase 2: Change Analysis
	pipe.Add(&changeAnalysisStep{
		rs:             rs,
		sessionService: w.sessionService,
	})

	// Phase 3: Analysis
	pipe.Add(&analysisStep{
		rs:             rs,
		sessionService: w.sessionService,
		llmProvider:    w.llmProvider,
		config:         w.config,
	})

	// Phase 4: Parallel Reviews
	pipe.Add(&parallelReviewsStep{
		rs:             rs,
		agentManager:   w.agentManager,
		sessionService: w.sessionService,
	})

	// Phase 5: Aggregation
	pipe.Add(&aggregationStep{
		rs:             rs,
		sessionService: w.sessionService,
	})

	// Phase 6: Discussion (if enabled)
	if w.config.EnableDiscussion {
		pipe.Add(&discussionStep{
			rs:               rs,
			discussionOrch:   w.discussionOrch,
			sessionService:   w.sessionService,
			discussionRounds: w.config.DiscussionRounds,
		})
	}

	// Phase 7: Synthesis
	pipe.Add(&synthesisStep{
		rs:                 rs,
		sessionService:     w.sessionService,
		enableRequirements: w.config.EnableRequirements,
		llmProvider:        w.llmProvider,
	})

	// Phase 8: Complete
	pipe.Add(&completeStep{
		rs:             rs,
		sessionService: w.sessionService,
	})

	return pipe
}

// failSession marks a session as failed
func (w *ReviewWorkflow) failSession(ctx context.Context, sessionID string, reason string) {
	if err := w.sessionService.FailSession(ctx, sessionID, reason); err != nil {
		logx.Errorf("fail session failed: %v", err)
	}
}
