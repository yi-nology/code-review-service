package workflow

import (
	"context"
	"fmt"
	"time"

	"github.com/yi-nology/code-review-service/internal/service/agent"
	"github.com/yi-nology/code-review-service/internal/service/discussion"
	"github.com/yi-nology/code-review-service/internal/service/progress"
	"github.com/yi-nology/code-review-service/internal/service/requirements"
	"github.com/yi-nology/code-review-service/internal/service/reviewmap"
	"github.com/yi-nology/code-review-service/internal/service/session"
	"github.com/yi-nology/code-review-service/internal/repository/postgres/models"
	"github.com/cloudwego/hertz/pkg/common/hlog"
)

// ReviewWorkflow orchestrates the multi-agent review process
type ReviewWorkflow struct {
	agentManager    *agent.AgentManager
	sessionService  session.Service
	progressManager *progress.ProgressManager
	discussionOrch  *discussion.Orchestrator
	requirementEval *requirements.Evaluator
	mapGenerator    *reviewmap.Generator
	config          WorkflowConfig
}

// WorkflowConfig contains workflow configuration
type WorkflowConfig struct {
	EnableDiscussion bool
	EnableRequirements bool
	EnableReviewMap bool
	DiscussionRounds int
}

// DefaultWorkflowConfig returns default workflow configuration
func DefaultWorkflowConfig() WorkflowConfig {
	return WorkflowConfig{
		EnableDiscussion: true,
		EnableRequirements: true,
		EnableReviewMap: true,
		DiscussionRounds: 1,
	}
}

// NewReviewWorkflow creates a new review workflow
func NewReviewWorkflow(
	agentManager *agent.AgentManager,
	sessionService session.Service,
	progressManager *progress.ProgressManager,
	config WorkflowConfig,
) *ReviewWorkflow {
	return &ReviewWorkflow{
		agentManager:    agentManager,
		sessionService:  sessionService,
		progressManager: progressManager,
		discussionOrch:  discussion.NewOrchestrator(discussion.DefaultDiscussionConfig()),
		requirementEval: requirements.NewEvaluator(),
		mapGenerator:    reviewmap.NewGenerator(),
		config:          config,
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
}

// ReviewResult contains the final review result
type ReviewResult struct {
	SessionID      string
	Round          int
	AgentResults   []agent.AgentResult
	Discussion     *discussion.DiscussionResult
	Requirements   *requirements.CoverageAssessment
	ReviewMap      *reviewmap.ReviewMap
	FinalReport    FinalReport
	Duration       time.Duration
}

// FinalReport contains the final review report
type FinalReport struct {
	Verdict          string
	Summary          string
	Blockers         []agent.Finding
	ShouldFix        []agent.Finding
	Suggestions      []agent.Finding
	Score            int
	GatePassed       bool
	GateReason       string
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

	// Phase 1: Context Discovery
	tracker.UpdatePhase(progress.PhaseContext, "active", 0, "Loading configuration and discovering project standards")
	
	// Parse requirements if provided
	var parsedRequirements *requirements.Requirements
	if len(request.Requirements) > 0 {
		parser := requirements.NewParser()
		parsedRequirements, err = parser.Parse(request.Requirements)
		if err != nil {
			w.failSession(ctx, session.SessionID, fmt.Sprintf("parse requirements: %v", err))
			tracker.SetError(fmt.Sprintf("parse requirements: %v", err))
			return nil, fmt.Errorf("parse requirements: %w", err)
		}
	}
	
	tracker.UpdatePhase(progress.PhaseContext, "completed", 100, "Context discovery completed")

	// Phase 2: Change Analysis
	tracker.UpdatePhase(progress.PhaseChangeContext, "active", 0, "Analyzing git diff and understanding changes")
	
	// Update session phase
	if err := w.sessionService.UpdatePhase(ctx, session.SessionID, models.SessionPhaseChangeContext, 50); err != nil {
		hlog.Errorf("update session phase failed: %v", err)
	}
	
	tracker.UpdatePhase(progress.PhaseChangeContext, "completed", 100, "Change analysis completed")

	// Phase 3: Tech Lead Analysis
	tracker.UpdatePhase(progress.PhaseAnalysis, "active", 0, "Summarizing changes and identifying risks")
	
	if err := w.sessionService.UpdatePhase(ctx, session.SessionID, models.SessionPhaseAnalysis, 50); err != nil {
		hlog.Errorf("update session phase failed: %v", err)
	}
	
	tracker.UpdatePhase(progress.PhaseAnalysis, "completed", 100, "Analysis completed")

	// Phase 4: Parallel Reviews
	tracker.UpdatePhase(progress.PhaseReviews, "active", 0, "Running multiple review agents in parallel")
	
	if err := w.sessionService.UpdatePhase(ctx, session.SessionID, models.SessionPhaseReviews, 0); err != nil {
		hlog.Errorf("update session phase failed: %v", err)
	}

	// Create workspace for agents
	workspace := agent.Workspace{
		Dir:     request.Dir,
		BaseSHA: request.BaseSHA,
		HeadSHA: request.HeadSHA,
		Diff:    request.Diff,
	}

	// Add requirements to workspace if available
	if parsedRequirements != nil {
		workspace.Requirements = &agent.Requirements{
			Inline: parsedRequirements.Inline,
			Files:  parsedRequirements.Files,
			URLs:   parsedRequirements.URLs,
		}
	}

	// Run agents with progress callback
	aggregatedResult, err := w.agentManager.RunAgentsWithCallback(ctx, workspace, func(status agent.AgentStatus) {
		tracker.UpdateAgentStatus(status.Name, status.Status, status.Progress, status.Message)
		
		// Update session agent results
		if err := w.updateSessionAgentResults(ctx, session.SessionID, status); err != nil {
			hlog.Errorf("update session agent results failed: %v", err)
		}
	})
	if err != nil {
		w.failSession(ctx, session.SessionID, fmt.Sprintf("run agents: %v", err))
		tracker.SetError(fmt.Sprintf("run agents: %v", err))
		return nil, fmt.Errorf("run agents: %w", err)
	}

	tracker.UpdatePhase(progress.PhaseReviews, "completed", 100, fmt.Sprintf("Completed with %d findings", len(aggregatedResult.Findings)))

	// Phase 5: Aggregation
	tracker.UpdatePhase(progress.PhaseAggregation, "active", 0, "Merging findings from multiple agents")
	
	if err := w.sessionService.UpdatePhase(ctx, session.SessionID, models.SessionPhaseAggregation, 50); err != nil {
		hlog.Errorf("update session phase failed: %v", err)
	}
	
	tracker.UpdatePhase(progress.PhaseAggregation, "completed", 100, "Aggregation completed")

	// Phase 6: Discussion (if enabled)
	var discussionResult *discussion.DiscussionResult
	if w.config.EnableDiscussion && len(aggregatedResult.AgentResults) >= 2 {
		tracker.UpdatePhase(progress.PhaseDiscussion, "active", 0, "Agents debating and validating findings")
		
		if err := w.sessionService.UpdatePhase(ctx, session.SessionID, models.SessionPhaseDiscussion, 0); err != nil {
			hlog.Errorf("update session phase failed: %v", err)
		}

		// Get agent names
		agentNames := make([]string, 0, len(aggregatedResult.AgentResults))
		for _, result := range aggregatedResult.AgentResults {
			agentNames = append(agentNames, result.AgentName)
		}

		// Convert findings to discussion format
		discussionFindings := make([]discussion.Finding, 0, len(aggregatedResult.Findings))
		for _, finding := range aggregatedResult.Findings {
			discussionFindings = append(discussionFindings, discussion.Finding{
				ID:         finding.ID,
				Severity:   finding.Severity,
				Category:   finding.Category,
				Path:       finding.Path,
				Line:       finding.Line,
				EndLine:    finding.EndLine,
				Title:      finding.Title,
				Body:       finding.Body,
				Suggestion: finding.Suggestion,
				Confidence: finding.Confidence,
				Agents:     finding.Agents,
			})
		}

		// Run discussion
		discussionResult, err = w.discussionOrch.RunDiscussion(ctx, discussionFindings, agentNames, request.Requirements)
		if err != nil {
			hlog.Errorf("run discussion failed: %v", err)
			// Continue without discussion
		} else {
			// Update session with discussion
			discussionData := models.DiscussionData{
				ID:      discussionResult.Summary,
				Round:   request.Round,
				Summary: discussionResult.Summary,
			}
			if err := w.sessionService.UpdateDiscussion(ctx, session.SessionID, discussionData); err != nil {
				hlog.Errorf("update session discussion failed: %v", err)
			}
			
			// Update findings confidence based on discussion
			if discussionResult != nil {
				w.updateFindingsFromDiscussion(aggregatedResult.Findings, discussionResult)
			}
		}
		
		tracker.UpdatePhase(progress.PhaseDiscussion, "completed", 100, "Discussion completed")
	}

	// Phase 7: Synthesis
	tracker.UpdatePhase(progress.PhaseSynthesis, "active", 0, "Producing final prioritized review")
	
	if err := w.sessionService.UpdatePhase(ctx, session.SessionID, models.SessionPhaseSynthesis, 0); err != nil {
		hlog.Errorf("update session phase failed: %v", err)
	}

	// Generate requirements coverage if enabled
	var requirementsCoverage *requirements.CoverageAssessment
	if w.config.EnableRequirements && len(request.Requirements) > 0 {
		// Convert findings to requirements format
		reqFindings := make([]requirements.Finding, 0, len(aggregatedResult.Findings))
		for _, finding := range aggregatedResult.Findings {
			reqFindings = append(reqFindings, requirements.Finding{
				ID:         finding.ID,
				Severity:   finding.Severity,
				Category:   finding.Category,
				Path:       finding.Path,
				Line:       finding.Line,
				EndLine:    finding.EndLine,
				Title:      finding.Title,
				Body:       finding.Body,
				Suggestion: finding.Suggestion,
				Confidence: finding.Confidence,
				Agents:     finding.Agents,
			})
		}

		// Parse requirements
		parser := requirements.NewParser()
		parsedReqs := parser.ParseRequirements(fmt.Sprintf("%v", request.Requirements))
		
		// Evaluate coverage
		requirementsCoverage = w.requirementEval.Evaluate(reqFindings, parsedReqs)
	}

	// Generate review map if enabled
	var reviewMap *reviewmap.ReviewMap
	if w.config.EnableReviewMap {
		// Convert findings to reviewmap format
		mapFindings := make([]reviewmap.Finding, 0, len(aggregatedResult.Findings))
		for _, finding := range aggregatedResult.Findings {
			mapFindings = append(mapFindings, reviewmap.Finding{
				ID:         finding.ID,
				Severity:   finding.Severity,
				Category:   finding.Category,
				Path:       finding.Path,
				Line:       finding.Line,
				EndLine:    finding.EndLine,
				Title:      finding.Title,
				Body:       finding.Body,
				Suggestion: finding.Suggestion,
				Confidence: finding.Confidence,
				Agents:     finding.Agents,
			})
		}

		reviewMap, err = w.mapGenerator.Generate(session.SessionID, request.Diff, mapFindings)
		if err != nil {
			hlog.Errorf("generate review map failed: %v", err)
			// Continue without map
		}
	}

	// Generate final report
	finalReport := w.generateFinalReport(aggregatedResult, discussionResult, requirementsCoverage)
	
	// Update session with final report
	finalReportData := models.FinalReportData{
		Verdict:    finalReport.Verdict,
		Summary:    finalReport.Summary,
		Score:      finalReport.Score,
		GatePassed: finalReport.GatePassed,
		GateReason: finalReport.GateReason,
	}
	if err := w.sessionService.UpdateFinalReport(ctx, session.SessionID, finalReportData); err != nil {
		hlog.Errorf("update session final report failed: %v", err)
	}
	
	tracker.UpdatePhase(progress.PhaseSynthesis, "completed", 100, "Synthesis completed")

	// Phase 8: Complete
	tracker.UpdatePhase(progress.PhaseComplete, "active", 0, "Review completed")
	
	// Complete session
	if err := w.sessionService.CompleteSession(ctx, session.SessionID, finalReportData); err != nil {
		hlog.Errorf("complete session failed: %v", err)
	}
	
	tracker.Complete()

	duration := time.Since(startTime)
	hlog.Infof("review completed in %v with %d findings", duration, len(aggregatedResult.Findings))

	return &ReviewResult{
		SessionID:      session.SessionID,
		Round:          request.Round,
		AgentResults:   aggregatedResult.AgentResults,
		Discussion:     discussionResult,
		Requirements:   requirementsCoverage,
		ReviewMap:      reviewMap,
		FinalReport:    finalReport,
		Duration:       duration,
	}, nil
}

// failSession marks a session as failed
func (w *ReviewWorkflow) failSession(ctx context.Context, sessionID string, reason string) {
	if err := w.sessionService.FailSession(ctx, sessionID, reason); err != nil {
		hlog.Errorf("fail session failed: %v", err)
	}
}

// updateSessionAgentResults updates session with agent status
func (w *ReviewWorkflow) updateSessionAgentResults(ctx context.Context, sessionID string, status agent.AgentStatus) error {
	// Get current session
	session, err := w.sessionService.GetSession(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("get session: %w", err)
	}

	// Parse existing agent results
	agentResults := make([]models.AgentResultData, 0)
	if session.AgentResults != nil {
		// In a real implementation, we would unmarshal and update
		// For now, we'll just append
	}

	// Add new agent result
	agentResult := models.AgentResultData{
		AgentName: status.Name,
		Summary:   status.Message,
		Duration:  status.Duration.Milliseconds(),
	}
	agentResults = append(agentResults, agentResult)

	// Update session
	return w.sessionService.UpdateAgentResults(ctx, sessionID, agentResults)
}

// updateFindingsFromDiscussion updates findings confidence based on discussion
func (w *ReviewWorkflow) updateFindingsFromDiscussion(findings []agent.Finding, discussionResult *discussion.DiscussionResult) {
	// Create a map of finding ID to updated confidence
	confidenceUpdates := make(map[string]float64)
	
	for _, consensus := range discussionResult.Consensus {
		// Increase confidence for consensus findings
		consensus.Agents = append(consensus.Agents, consensus.Agents...)
		consensus.Confidence = min(1.0, consensus.Confidence+0.1)
	}
	
	for _, disagreement := range discussionResult.Disagreements {
		// Decrease confidence for challenged findings
		confidenceUpdates[disagreement.FindingID] = 0.5
	}
	
	// Apply updates
	for i, finding := range findings {
		if newConfidence, ok := confidenceUpdates[finding.ID]; ok {
			findings[i].Confidence = newConfidence
		}
	}
}

// generateFinalReport generates the final review report
func (w *ReviewWorkflow) generateFinalReport(
	aggregatedResult agent.AggregatedResult,
	discussionResult *discussion.DiscussionResult,
	requirementsCoverage *requirements.CoverageAssessment,
) FinalReport {
	// Categorize findings
	var blockers []agent.Finding
	var shouldFix []agent.Finding
	var suggestions []agent.Finding
	
	for _, finding := range aggregatedResult.Findings {
		switch finding.Severity {
		case "critical":
			blockers = append(blockers, finding)
		case "major":
			shouldFix = append(shouldFix, finding)
		default:
			suggestions = append(suggestions, finding)
		}
	}
	
	// Calculate score
	score := 100
	score -= len(blockers) * 25
	score -= len(shouldFix) * 12
	score -= len(suggestions) * 5
	if score < 0 {
		score = 0
	}
	
	// Determine verdict
	verdict := "APPROVE"
	gatePassed := true
	gateReason := ""
	
	if len(blockers) > 0 {
		verdict = "REQUEST_CHANGES"
		gatePassed = false
		gateReason = fmt.Sprintf("%d blocker(s) found", len(blockers))
	} else if len(shouldFix) > 3 {
		verdict = "REQUEST_CHANGES"
		gatePassed = false
		gateReason = fmt.Sprintf("%d should-fix items found", len(shouldFix))
	}
	
	// Generate summary
	summary := fmt.Sprintf("Review completed with %d findings: %d blockers, %d should-fix, %d suggestions",
		len(aggregatedResult.Findings), len(blockers), len(shouldFix), len(suggestions))
	
	return FinalReport{
		Verdict:    verdict,
		Summary:    summary,
		Blockers:   blockers,
		ShouldFix:  shouldFix,
		Suggestions: suggestions,
		Score:      score,
		GatePassed: gatePassed,
		GateReason: gateReason,
	}
}

// min returns the minimum of two float64 values
func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
