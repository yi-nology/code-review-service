package workflow

import (
	"context"
	"fmt"

	"github.com/yi-nology/code-review-service/internal/repository/postgres/models"
	"github.com/yi-nology/code-review-service/internal/service/agent"
	"github.com/yi-nology/code-review-service/internal/service/discussion"
	"github.com/yi-nology/code-review-service/internal/service/progress"
	"github.com/yi-nology/code-review-service/internal/service/requirements"
	"github.com/yi-nology/code-review-service/internal/service/reviewmap"
	"github.com/oekit/workflow"
	"github.com/cloudwego/hertz/pkg/common/hlog"
)

// toReviewSession 安全地将 Session 转换为 ReviewSession
func toReviewSession(sess workflow.Session) (*ReviewSession, error) {
	rs, ok := sess.(*ReviewSession)
	if !ok {
		return nil, fmt.Errorf("invalid session type: expected *ReviewSession")
	}
	return rs, nil
}

// contextDiscoveryStep Phase 1: 加载配置和发现项目标准
type contextDiscoveryStep struct{}

func (s *contextDiscoveryStep) Run(ctx context.Context, req workflow.Request, sess workflow.Session) error {
	rs, err := toReviewSession(sess)
	if err != nil {
		return err
	}
	rs.Tracker.UpdatePhase(progress.PhaseContext, "active", 0, "Loading configuration and discovering project standards")

	if len(rs.Request.Requirements) > 0 {
		parser := requirements.NewParser()
		parsedReqs, err := parser.Parse(rs.Request.Requirements)
		if err != nil {
			return fmt.Errorf("parse requirements: %w", err)
		}
		rs.ParsedReqs = parsedReqs
	}

	rs.Tracker.UpdatePhase(progress.PhaseContext, "completed", 100, "Context discovery completed")
	return nil
}

// changeAnalysisStep Phase 2: 分析 git diff 和理解变更
type changeAnalysisStep struct {
	sessionService sessionService
}

func (s *changeAnalysisStep) Run(ctx context.Context, req workflow.Request, sess workflow.Session) error {
	rs, err := toReviewSession(sess)
	if err != nil {
		return err
	}
	rs.Tracker.UpdatePhase(progress.PhaseChangeContext, "active", 0, "Analyzing git diff and understanding changes")

	if err := s.sessionService.UpdatePhase(ctx, rs.SessionID, models.SessionPhaseChangeContext, 50); err != nil {
		hlog.Errorf("update session phase failed: %v", err)
	}

	rs.Tracker.UpdatePhase(progress.PhaseChangeContext, "completed", 100, "Change analysis completed")
	return nil
}

// analysisStep Phase 3: 总结变更和识别风险
type analysisStep struct {
	sessionService sessionService
}

func (s *analysisStep) Run(ctx context.Context, req workflow.Request, sess workflow.Session) error {
	rs, err := toReviewSession(sess)
	if err != nil {
		return err
	}
	rs.Tracker.UpdatePhase(progress.PhaseAnalysis, "active", 0, "Summarizing changes and identifying risks")

	if err := s.sessionService.UpdatePhase(ctx, rs.SessionID, models.SessionPhaseAnalysis, 50); err != nil {
		hlog.Errorf("update session phase failed: %v", err)
	}

	rs.Tracker.UpdatePhase(progress.PhaseAnalysis, "completed", 100, "Analysis completed")
	return nil
}

// parallelReviewsStep Phase 4: 并行运行多个 review agents
type parallelReviewsStep struct {
	agentManager   *agent.AgentManager
	sessionService sessionService
}

func (s *parallelReviewsStep) Run(ctx context.Context, req workflow.Request, sess workflow.Session) error {
	rs, err := toReviewSession(sess)
	if err != nil {
		return err
	}
	rs.Tracker.UpdatePhase(progress.PhaseReviews, "active", 0, "Running multiple review agents in parallel")

	if err := s.sessionService.UpdatePhase(ctx, rs.SessionID, models.SessionPhaseReviews, 0); err != nil {
		hlog.Errorf("update session phase failed: %v", err)
	}

	// 准备 workspace
	workspace := agent.Workspace{
		Dir:     rs.Request.Dir,
		BaseSHA: rs.Request.BaseSHA,
		HeadSHA: rs.Request.HeadSHA,
		Diff:    rs.Request.Diff,
	}

	// 添加 requirements 到 workspace
	if rs.ParsedReqs != nil {
		workspace.Requirements = &agent.Requirements{
			Inline: rs.ParsedReqs.Inline,
			Files:  rs.ParsedReqs.Files,
			URLs:   rs.ParsedReqs.URLs,
		}
	}

	rs.Workspace = workspace

	// 运行 agents
	aggregatedResult, err := s.agentManager.RunAgentsWithCallback(ctx, workspace, func(status agent.AgentStatus) {
		rs.Tracker.UpdateAgentStatus(status.Name, status.Status, status.Progress, status.Message)
	})

	if err != nil {
		return fmt.Errorf("run agents: %w", err)
	}

	rs.AggregatedResult = &aggregatedResult
	rs.Tracker.UpdatePhase(progress.PhaseReviews, "completed", 100, fmt.Sprintf("Completed with %d findings", len(aggregatedResult.Findings)))
	return nil
}

// aggregationStep Phase 5: 合并来自多个 agents 的 findings
type aggregationStep struct {
	sessionService sessionService
}

func (s *aggregationStep) Run(ctx context.Context, req workflow.Request, sess workflow.Session) error {
	rs, err := toReviewSession(sess)
	if err != nil {
		return err
	}
	rs.Tracker.UpdatePhase(progress.PhaseAggregation, "active", 0, "Merging findings from multiple agents")

	if err := s.sessionService.UpdatePhase(ctx, rs.SessionID, models.SessionPhaseAggregation, 50); err != nil {
		hlog.Errorf("update session phase failed: %v", err)
	}

	rs.Tracker.UpdatePhase(progress.PhaseAggregation, "completed", 100, "Aggregation completed")
	return nil
}

// discussionStep Phase 6: Agents 讨论和验证 findings
type discussionStep struct {
	discussionOrch   *discussion.Orchestrator
	sessionService   sessionService
	discussionRounds int
}

func (s *discussionStep) Run(ctx context.Context, req workflow.Request, sess workflow.Session) error {
	rs, err := toReviewSession(sess)
	if err != nil {
		return err
	}

	// 检查是否有足够的 agents 进行讨论
	if rs.AggregatedResult == nil || len(rs.AggregatedResult.AgentResults) < 2 {
		hlog.Info("skipping discussion: not enough agents")
		return nil
	}

	rs.Tracker.UpdatePhase(progress.PhaseDiscussion, "active", 0, "Agents debating and validating findings")

	if err := s.sessionService.UpdatePhase(ctx, rs.SessionID, models.SessionPhaseDiscussion, 0); err != nil {
		hlog.Errorf("update session phase failed: %v", err)
	}

	// 获取 agent 名称
	agentNames := make([]string, 0, len(rs.AggregatedResult.AgentResults))
	for _, result := range rs.AggregatedResult.AgentResults {
		agentNames = append(agentNames, result.AgentName)
	}

	// 转换 findings 到 discussion 格式
	discussionFindings := make([]discussion.Finding, len(rs.AggregatedResult.Findings))
	copy(discussionFindings, rs.AggregatedResult.Findings)

	// 运行讨论
	discussionResult, err := s.discussionOrch.RunDiscussion(ctx, discussionFindings, agentNames, rs.Request.Requirements)
	switch {
	case err != nil:
		hlog.Errorf("run discussion failed: %v", err)
	case discussionResult != nil:
		rs.DiscussionResult = discussionResult
		// 更新 findings confidence
		updateFindingsFromDiscussion(rs.AggregatedResult.Findings, discussionResult)
	default:
		hlog.Warn("run discussion returned empty result")
	}

	rs.Tracker.UpdatePhase(progress.PhaseDiscussion, "completed", 100, "Discussion completed")
	return nil
}

// synthesisStep Phase 7: 生成最终报告
type synthesisStep struct {
	sessionService     sessionService
	enableRequirements bool
	enableReviewMap    bool
}

func (s *synthesisStep) Run(ctx context.Context, req workflow.Request, sess workflow.Session) error {
	rs, err := toReviewSession(sess)
	if err != nil {
		return err
	}
	rs.Tracker.UpdatePhase(progress.PhaseSynthesis, "active", 0, "Producing final prioritized review")

	if err := s.sessionService.UpdatePhase(ctx, rs.SessionID, models.SessionPhaseSynthesis, 0); err != nil {
		hlog.Errorf("update session phase failed: %v", err)
	}

	// 生成 requirements coverage
	if s.enableRequirements && len(rs.Request.Requirements) > 0 && rs.AggregatedResult != nil {
		reqFindings := make([]requirements.Finding, len(rs.AggregatedResult.Findings))
		copy(reqFindings, rs.AggregatedResult.Findings)

		parser := requirements.NewParser()
		parsedReqs := parser.ParseRequirements(fmt.Sprintf("%v", rs.Request.Requirements))
		requirementEval := requirements.NewEvaluator()
		rs.Requirements = requirementEval.Evaluate(reqFindings, parsedReqs)
	}

	// 生成 review map
	if s.enableReviewMap && rs.AggregatedResult != nil {
		mapFindings := make([]reviewmap.Finding, len(rs.AggregatedResult.Findings))
		copy(mapFindings, rs.AggregatedResult.Findings)

		mapGenerator := reviewmap.NewGenerator()
		reviewMap, err := mapGenerator.Generate(rs.SessionID, rs.Request.Diff, mapFindings)
		if err != nil {
			hlog.Errorf("generate review map failed: %v", err)
		} else {
			rs.ReviewMap = reviewMap
		}
	}

	// 生成最终报告
	if rs.AggregatedResult != nil {
		report := generateFinalReport(*rs.AggregatedResult, rs.DiscussionResult, rs.Requirements)
		rs.FinalReport = &report

		// 更新 session
		finalReportData := models.FinalReportData{
			Verdict:    report.Verdict,
			Summary:    report.Summary,
			Score:      report.Score,
			GatePassed: report.GatePassed,
			GateReason: report.GateReason,
		}
		if err := s.sessionService.UpdateFinalReport(ctx, rs.SessionID, finalReportData); err != nil {
			hlog.Errorf("update session final report failed: %v", err)
		}
	}

	rs.Tracker.UpdatePhase(progress.PhaseSynthesis, "completed", 100, "Synthesis completed")
	return nil
}

// completeStep Phase 8: 完成 review
type completeStep struct {
	sessionService sessionService
}

func (s *completeStep) Run(ctx context.Context, req workflow.Request, sess workflow.Session) error {
	rs, err := toReviewSession(sess)
	if err != nil {
		return err
	}
	rs.Tracker.UpdatePhase(progress.PhaseComplete, "active", 0, "Review completed")

	if rs.FinalReport != nil {
		finalReportData := models.FinalReportData{
			Verdict:    rs.FinalReport.Verdict,
			Summary:    rs.FinalReport.Summary,
			Score:      rs.FinalReport.Score,
			GatePassed: rs.FinalReport.GatePassed,
			GateReason: rs.FinalReport.GateReason,
		}
		if err := s.sessionService.CompleteSession(ctx, rs.SessionID, finalReportData); err != nil {
			hlog.Errorf("complete session failed: %v", err)
		}
	}

	rs.Tracker.Complete()
	return nil
}

// updateFindingsFromDiscussion 根据讨论结果更新 findings 的 confidence
func updateFindingsFromDiscussion(findings []agent.Finding, discussionResult *discussion.DiscussionResult) {
	confidenceUpdates := make(map[string]float64)

	for _, disagreement := range discussionResult.Disagreements {
		confidenceUpdates[disagreement.FindingID] = 0.5
	}

	for i, finding := range findings {
		if newConfidence, ok := confidenceUpdates[finding.ID]; ok {
			findings[i].Confidence = newConfidence
		}
	}
}

// generateFinalReport 生成最终报告
func generateFinalReport(
	aggregatedResult agent.AggregatedResult,
	discussionResult *discussion.DiscussionResult,
	requirementsCoverage *requirements.CoverageAssessment,
) FinalReport {
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

	score := 100
	score -= len(blockers) * 25
	score -= len(shouldFix) * 12
	score -= len(suggestions) * 5
	if score < 0 {
		score = 0
	}

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

	summary := fmt.Sprintf("Review completed with %d findings: %d blockers, %d should-fix, %d suggestions",
		len(aggregatedResult.Findings), len(blockers), len(shouldFix), len(suggestions))

	return FinalReport{
		Verdict:     verdict,
		Summary:     summary,
		Blockers:    blockers,
		ShouldFix:   shouldFix,
		Suggestions: suggestions,
		Score:       score,
		GatePassed:  gatePassed,
		GateReason:  gateReason,
	}
}

// sessionService 是 session.Service 的本地接口，避免循环依赖
type sessionService interface {
	CreateSession(ctx context.Context, reviewID string, round int) (*models.ReviewSession, error)
	UpdatePhase(ctx context.Context, sessionID string, phase models.SessionPhase, progress float64) error
	UpdateAgentResults(ctx context.Context, sessionID string, agentResults []models.AgentResultData) error
	UpdateDiscussion(ctx context.Context, sessionID string, discussion models.DiscussionData) error
	UpdateFinalReport(ctx context.Context, sessionID string, finalReport models.FinalReportData) error
	CompleteSession(ctx context.Context, sessionID string, finalReport models.FinalReportData) error
	FailSession(ctx context.Context, sessionID string, reason string) error
}
