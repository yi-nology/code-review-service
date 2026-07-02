package workflow

import (
	"context"
	"fmt"
	"time"

	"github.com/yi-nology/code-review-service/internal/ocr"
	"github.com/yi-nology/code-review-service/internal/service/agent"
	"github.com/yi-nology/code-review-service/internal/service/discussion"
	"github.com/yi-nology/code-review-service/internal/service/progress"
	"github.com/oekit/logx"
	"github.com/oekit/workflow"
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
	ContextWindow      int // 上下文窗口大小（tokens），0=模型默认
}

// GetMaxDiffChars 根据上下文窗口大小计算合适的 diff 截断字符数
// 经验公式：diff 应占上下文窗口的 30-40%，留空间给系统提示、用户提示和输出
func (c *WorkflowConfig) GetMaxDiffChars() int {
	if c.ContextWindow <= 0 {
		// 未配置上下文窗口，使用默认值
		return 30000
	}
	// 按上下文窗口的 35% 计算 diff 上限，但不少于 10000，不超过 500000
	limit := int(float64(c.ContextWindow) * 0.35 / 4) // 假设平均 4 chars/token
	if limit < 10000 {
		limit = 10000
	}
	if limit > 500000 {
		limit = 500000
	}
	return limit
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
	}

	// Build and run pipeline
	pipe := w.buildPipeline()
	if err := pipe.Run(ctx, request, rs); err != nil {
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

// buildPipeline 构建 review 流程管道
func (w *ReviewWorkflow) buildPipeline() *workflow.PipeFlow {
	pipe := workflow.NewPipe()

	// Phase 1: Context Discovery
	pipe.Add(&contextDiscoveryStep{})

	// Phase 2: Change Analysis
	pipe.Add(&changeAnalysisStep{
		sessionService: w.sessionService,
	})

	// Phase 3: Analysis
	pipe.Add(&analysisStep{
		sessionService: w.sessionService,
		llmProvider:    w.llmProvider,
		config:         w.config,
	})

	// Phase 4: Parallel Reviews
	pipe.Add(&parallelReviewsStep{
		agentManager:   w.agentManager,
		sessionService: w.sessionService,
	})

	// Phase 5: Aggregation
	pipe.Add(&aggregationStep{
		sessionService: w.sessionService,
	})

	// Phase 6: Discussion (if enabled)
	if w.config.EnableDiscussion {
		pipe.Add(&discussionStep{
			discussionOrch:   w.discussionOrch,
			sessionService:   w.sessionService,
			discussionRounds: w.config.DiscussionRounds,
		})
	}

	// Phase 7: Synthesis
	pipe.Add(&synthesisStep{
		sessionService:     w.sessionService,
		enableRequirements: w.config.EnableRequirements,
		llmProvider:        w.llmProvider,
	})

	// Phase 8: Complete
	pipe.Add(&completeStep{
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
