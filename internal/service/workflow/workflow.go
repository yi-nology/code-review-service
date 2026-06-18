package workflow

import (
	"context"
	"fmt"
	"time"

	"github.com/yi-nology/code-review-service/internal/service/agent"
	"github.com/yi-nology/code-review-service/internal/service/discussion"
	"github.com/yi-nology/code-review-service/internal/service/progress"
	"github.com/oekit/workflow"
	"github.com/cloudwego/hertz/pkg/common/hlog"
)

// ReviewWorkflow orchestrates the multi-agent review process
type ReviewWorkflow struct {
	agentManager    *agent.AgentManager
	sessionService  sessionService
	progressManager *progress.ProgressManager
	discussionOrch  *discussion.Orchestrator
	config          WorkflowConfig
}

// WorkflowConfig contains workflow configuration
type WorkflowConfig struct {
	EnableDiscussion   bool
	EnableRequirements bool
	EnableReviewMap    bool
	DiscussionRounds   int
}

// DefaultWorkflowConfig returns default workflow configuration
func DefaultWorkflowConfig() WorkflowConfig {
	return WorkflowConfig{
		EnableDiscussion:   true,
		EnableRequirements: true,
		EnableReviewMap:    true,
		DiscussionRounds:   1,
	}
}

// NewReviewWorkflow creates a new review workflow
func NewReviewWorkflow(
	agentManager *agent.AgentManager,
	sessionService sessionService,
	progressManager *progress.ProgressManager,
	config WorkflowConfig,
) *ReviewWorkflow {
	return &ReviewWorkflow{
		agentManager:    agentManager,
		sessionService:  sessionService,
		progressManager: progressManager,
		discussionOrch:  discussion.NewOrchestrator(discussion.DefaultDiscussionConfig()),
		config:          config,
	}
}

// RunReview runs the complete review workflow
func (w *ReviewWorkflow) RunReview(ctx context.Context, request ReviewRequest) (*ReviewResult, error) {
	startTime := time.Now()

	// Create session
	session, err := w.sessionService.CreateSession(ctx, request.ReviewID, request.Round)
	if err != nil {
		return nil, fmt.Errorf("create session: %w", err)
	}

	// Create progress tracker
	tracker := w.progressManager.CreateTracker(request.ReviewID, session.SessionID, request.Round)
	defer w.progressManager.RemoveTracker(request.ReviewID)

	// Build review session
	rs := &ReviewSession{
		ReviewID:  request.ReviewID,
		SessionID: session.SessionID,
		Round:     request.Round,
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
		Round:     request.Round,
		Duration:  time.Since(startTime),
	}

	if rs.AggregatedResult != nil {
		result.AgentResults = rs.AggregatedResult.AgentResults
	}
	result.Discussion = rs.DiscussionResult
	result.Requirements = rs.Requirements
	result.ReviewMap = rs.ReviewMap
	if rs.FinalReport != nil {
		result.FinalReport = *rs.FinalReport
	}

	hlog.Infof("review completed in %v with %d findings", result.Duration, len(result.AgentResults))
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
		enableReviewMap:    w.config.EnableReviewMap,
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
		hlog.Errorf("fail session failed: %v", err)
	}
}
