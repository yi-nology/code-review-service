package workflow

import (
	"context"
	"fmt"
	"strings"

	dsession "github.com/yi-nology/code-review-service/internal/domain/session"
	"github.com/yi-nology/code-review-service/internal/ocr"
	"github.com/yi-nology/code-review-service/internal/service/agent"
	"github.com/yi-nology/code-review-service/internal/service/discussion"
	"github.com/yi-nology/code-review-service/internal/service/progress"
	"github.com/yi-nology/code-review-service/internal/service/requirements"
	"github.com/yi-nology/code-review-service/internal/service/scoring"
	"github.com/yi-nology/code-review-service/internal/service/session"
	"github.com/yi-nology/code-review-service/internal/service/types"
	"github.com/oekit/logx"
	"github.com/oekit/pkg/slicex"
)

// contextDiscoveryStep Phase 1: 加载配置和发现项目标准
type contextDiscoveryStep struct {
	rs *ReviewSession
}

func (s *contextDiscoveryStep) Run(ctx context.Context) error {
	rs := s.rs
	rs.Tracker.UpdatePhase(progress.PhaseContext, "active", 0, "Loading configuration and discovering project standards")

	if len(rs.Request.Requirements) > 0 {
		parser := requirements.NewParser()
		parsedReqs, err := parser.Parse(rs.Request.Requirements)
		if err != nil {
			return fmt.Errorf("parse requirements: %w", err)
		}
		rs.ParsedReqs = parsedReqs
	}

	// 阶段 5：发现项目约定文件（AGENTS.md / CLAUDE.md / .cursorrules 等）注入上下文
	if standards := discoverProjectStandards(rs.Request.Dir); len(standards) > 0 {
		if rs.Workspace.Context == nil {
			rs.Workspace.Context = make(map[string]interface{})
		}
		rs.Workspace.Context["project_standards"] = standards
	}

	rs.Tracker.UpdatePhase(progress.PhaseContext, "completed", 100, "Context discovery completed")
	return nil
}

// changeAnalysisStep Phase 2: 分析 git diff 和理解变更
type changeAnalysisStep struct {
	rs *ReviewSession
	sessionService sessionService
}

func (s *changeAnalysisStep) Run(ctx context.Context) error {
	rs := s.rs
	rs.Tracker.UpdatePhase(progress.PhaseChangeContext, "active", 0, "Analyzing git diff and understanding changes")

	if err := s.sessionService.UpdatePhase(ctx, rs.SessionID, dsession.PhaseChangeContext, 50); err != nil {
		logx.Errorf("update session phase failed: %v", err)
	}

	rs.Tracker.UpdatePhase(progress.PhaseChangeContext, "completed", 100, "Change analysis completed")
	return nil
}

// analysisStep Phase 3: Tech Lead 分析（B2）——总结变更、识别风险
type analysisStep struct {
	rs *ReviewSession
	sessionService sessionService
	llmProvider    ocr.Provider
	config         WorkflowConfig
}

func (s *analysisStep) Run(ctx context.Context) error {
	rs := s.rs
	rs.Tracker.UpdatePhase(progress.PhaseAnalysis, "active", 0, "Summarizing changes and identifying risks")

	// B2: 用 LLM 做真 Tech Lead 分析；不可用则降级（仅更新进度）
	if analysis := s.runTechLeadAnalysis(ctx, rs); analysis != nil {
		rs.TechLeadAnalysis = analysis
		logx.Infof("tech lead analysis produced: %d risks, %d recommendations", len(analysis.Risks), len(analysis.Recommendations))
	}

	if err := s.sessionService.UpdatePhase(ctx, rs.SessionID, dsession.PhaseAnalysis, 50); err != nil {
		logx.Errorf("update session phase failed: %v", err)
	}

	rs.Tracker.UpdatePhase(progress.PhaseAnalysis, "completed", 100, "Analysis completed")
	return nil
}

// runTechLeadAnalysis 调用 LLM 产出 Tech Lead 分析（B2）。LLM 不可用时返回 nil（降级）。
func (s *analysisStep) runTechLeadAnalysis(ctx context.Context, rs *ReviewSession) *TechLeadAnalysis {
	if s.llmProvider == nil || !s.llmProvider.Available(ctx) {
		return nil
	}
	if strings.TrimSpace(rs.Request.Diff) == "" {
		return nil
	}

	diff := rs.Request.Diff
	maxDiffChars := s.config.GetMaxDiffChars()
	if len(diff) > maxDiffChars {
		diff = diff[:maxDiffChars] + "\n... (diff truncated)"
	}

	language := strings.TrimSpace(rs.Request.Language)
	systemPrompt, sectionFiles, sectionDiff, sectionReqs, returnHint := techLeadPrompts(language)

	// A3：从 diff 提取变更文件清单作为显式范围（无需调二进制 preview 即可获得范围感知）
	changedFiles := extractChangedFiles(rs.Request.Diff)

	var sb strings.Builder
	if len(changedFiles) > 0 {
		sb.WriteString(sectionFiles + "\n")
		for _, f := range changedFiles {
			sb.WriteString("- ")
			sb.WriteString(f)
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}
	sb.WriteString(sectionDiff + "\n```\n")
	sb.WriteString(diff)
	sb.WriteString("\n```\n")
	if len(rs.Request.Requirements) > 0 {
		sb.WriteString("\n" + sectionReqs + "\n")
		for _, r := range rs.Request.Requirements {
			sb.WriteString("- ")
			sb.WriteString(r)
			sb.WriteString("\n")
		}
	}
	sb.WriteString("\n" + returnHint)

	var analysis TechLeadAnalysis
	if err := ocr.CompleteJSON(ctx, s.llmProvider, ocr.CompleteRequest{
		SystemPrompt: systemPrompt,
		UserPrompt:   sb.String(),
		MaxTokens:    2048,
	}, &analysis); err != nil {
		logx.Warnf("tech lead LLM analysis failed (degraded): %v", err)
		return nil
	}
	return &analysis
}

// techLeadPrompts 根据语言返回 Tech Lead 分析的 prompt 各部分。
// language 为空或 "Chinese" 时使用中文，否则使用英文。
func techLeadPrompts(language string) (systemPrompt, sectionFiles, sectionDiff, sectionReqs, returnHint string) {
	if language == "" || strings.EqualFold(language, "Chinese") {
		return "你是一名资深技术负责人（Tech Lead），负责分析代码变更。请基于 diff 产出结构化分析：变更摘要、风险点清单、改进建议。只返回 JSON，不要任何解释或 markdown 围栏。",
			"【变更文件范围】",
			"【代码变更 diff】",
			"【需求上下文】",
			"请返回 JSON：{\"summary\": \"变更摘要\", \"risks\": [\"风险点\"], \"recommendations\": [\"改进建议\"]}"
	}
	return "You are a senior Tech Lead responsible for analyzing code changes. Produce a structured analysis based on the diff: change summary, risk list, and improvement suggestions. Return JSON only, no explanations or markdown fences.",
		"【Changed Files】",
		"【Code Diff】",
		"【Requirements Context】",
		"Return JSON: {\"summary\": \"change summary\", \"risks\": [\"risk items\"], \"recommendations\": [\"improvement suggestions\"]}"
}

// parallelReviewsStep Phase 4: 并行运行多个 review agents
type parallelReviewsStep struct {
	rs *ReviewSession
	agentManager   *agent.AgentManager
	sessionService sessionService
}

func (s *parallelReviewsStep) Run(ctx context.Context) error {
	rs := s.rs
	rs.Tracker.UpdatePhase(progress.PhaseReviews, "active", 0, "Running multiple review agents in parallel")

	if err := s.sessionService.UpdatePhase(ctx, rs.SessionID, dsession.PhaseReviews, 0); err != nil {
		logx.Errorf("update session phase failed: %v", err)
	}

	// 准备 workspace
	workspace := agent.Workspace{
		Dir:      rs.Request.Dir,
		BaseSHA:  rs.Request.BaseSHA,
		HeadSHA:  rs.Request.HeadSHA,
		Diff:     rs.Request.Diff,
		RulePath: rs.Request.RulePath,
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

	// 运行 agents（含 ephemeral，阶段 5）；按项目裁剪（per-project 白名单 / 技术栈自动推断）
	aggregatedResult, err := s.agentManager.RunAgentsForProject(ctx, workspace, rs.Request.Provider, rs.Request.Repo, rs.Request.EphemeralReviewers, func(status agent.AgentStatus) {
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
	rs *ReviewSession
	sessionService sessionService
}

func (s *aggregationStep) Run(ctx context.Context) error {
	rs := s.rs
	rs.Tracker.UpdatePhase(progress.PhaseAggregation, "active", 0, "Merging findings from multiple agents")

	if err := s.sessionService.UpdatePhase(ctx, rs.SessionID, dsession.PhaseAggregation, 50); err != nil {
		logx.Errorf("update session phase failed: %v", err)
	}

	rs.Tracker.UpdatePhase(progress.PhaseAggregation, "completed", 100, "Aggregation completed")
	return nil
}

// discussionStep Phase 6: Agents 讨论和验证 findings
type discussionStep struct {
	rs *ReviewSession
	discussionOrch   *discussion.Orchestrator
	sessionService   sessionService
	discussionRounds int
}

func (s *discussionStep) Run(ctx context.Context) error {
	rs := s.rs

	// 检查是否有足够的 agents 进行讨论
	if rs.AggregatedResult == nil || len(rs.AggregatedResult.AgentResults) < 2 {
		logx.Info("skipping discussion: not enough agents")
		return nil
	}

	rs.Tracker.UpdatePhase(progress.PhaseDiscussion, "active", 0, "Agents debating and validating findings")

	if err := s.sessionService.UpdatePhase(ctx, rs.SessionID, dsession.PhaseDiscussion, 0); err != nil {
		logx.Errorf("update session phase failed: %v", err)
	}

	// 获取 agent 名称
	agentNames := slicex.Map(rs.AggregatedResult.AgentResults, func(r agent.AgentResult) string { return r.AgentName })

	// 转换 findings 到 discussion 格式
	discussionFindings := make([]discussion.Finding, len(rs.AggregatedResult.Findings))
	copy(discussionFindings, rs.AggregatedResult.Findings)

	// 运行讨论
	discussionResult, err := s.discussionOrch.RunDiscussion(ctx, discussionFindings, agentNames, rs.Request.Requirements)
	switch {
	case err != nil:
		logx.Errorf("run discussion failed: %v", err)
	case discussionResult != nil:
		rs.DiscussionResult = discussionResult
		// 更新 findings confidence
		updateFindingsFromDiscussion(rs.AggregatedResult.Findings, discussionResult)
	default:
		logx.Warn("run discussion returned empty result")
	}

	rs.Tracker.UpdatePhase(progress.PhaseDiscussion, "completed", 100, "Discussion completed")
	return nil
}

// synthesisStep Phase 7: 生成最终报告
type synthesisStep struct {
	rs *ReviewSession
	sessionService     sessionService
	enableRequirements bool
	llmProvider        ocr.Provider
}

func (s *synthesisStep) Run(ctx context.Context) error {
	rs := s.rs
	rs.Tracker.UpdatePhase(progress.PhaseSynthesis, "active", 0, "Producing final prioritized review")

	if err := s.sessionService.UpdatePhase(ctx, rs.SessionID, dsession.PhaseSynthesis, 0); err != nil {
		logx.Errorf("update session phase failed: %v", err)
	}

	// 生成 requirements coverage
	if s.enableRequirements && len(rs.Request.Requirements) > 0 && rs.AggregatedResult != nil {
		reqFindings := make([]requirements.Finding, len(rs.AggregatedResult.Findings))
		copy(reqFindings, rs.AggregatedResult.Findings)

		parser := requirements.NewParser()
		parsedReqs := parser.ParseRequirements(strings.Join(rs.Request.Requirements, "\n"))
		requirementEval := requirements.NewEvaluator()
		rs.Requirements = requirementEval.Evaluate(reqFindings, parsedReqs)
	}

	// 生成最终报告
	if rs.AggregatedResult != nil {
		changedLines := scoring.CountDiffLines(rs.Request.Diff)
		report := generateFinalReport(*rs.AggregatedResult, rs.DiscussionResult, rs.Requirements, changedLines)

		// 阶段 4：用 LLM 生成澄清问题（需求歧义/范围/验收/边缘场景），LLM 不可用时为空
		if len(rs.Request.Requirements) > 0 {
			reqFindings := make([]requirements.Finding, len(rs.AggregatedResult.Findings))
			copy(reqFindings, rs.AggregatedResult.Findings)
			questions := requirements.GenerateClarifyingQuestions(ctx, s.llmProvider, rs.Request.Requirements, reqFindings)
			report.ClarifyingQuestions = toWorkflowQuestions(questions)
		}

		rs.FinalReport = &report

		// 更新 session（持久化完整报告，阶段 4/B2）
		finalReportData := dsession.FinalReportData{
			Verdict:     report.Verdict,
			Summary:     report.Summary,
			Score:       report.Score,
			GatePassed:  report.GatePassed,
			GateReason:  report.GateReason,
			Blockers:    toFindingDataSlice(report.Blockers),
			ShouldFix:   toFindingDataSlice(report.ShouldFix),
			Suggestions: toFindingDataSlice(report.Suggestions),
		}
		if len(report.ClarifyingQuestions) > 0 {
			finalReportData.ClarifyingQuestions = make([]dsession.ClarifyingQuestionData, len(report.ClarifyingQuestions))
			for i, q := range report.ClarifyingQuestions {
				finalReportData.ClarifyingQuestions[i] = dsession.ClarifyingQuestionData{
					Category: q.Category, Question: q.Question, Context: q.Context,
				}
			}
		}
		if rs.TechLeadAnalysis != nil {
			finalReportData.TechLeadAnalysis = &dsession.TechLeadAnalysisData{
				Summary:         rs.TechLeadAnalysis.Summary,
				Risks:           rs.TechLeadAnalysis.Risks,
				Recommendations: rs.TechLeadAnalysis.Recommendations,
			}
		}
		if err := s.sessionService.UpdateFinalReport(ctx, rs.SessionID, finalReportData); err != nil {
			logx.Errorf("update session final report failed: %v", err)
		}
	}

	rs.Tracker.UpdatePhase(progress.PhaseSynthesis, "completed", 100, "Synthesis completed")
	return nil
}

// completeStep Phase 8: 完成 review
type completeStep struct {
	rs *ReviewSession
	sessionService sessionService
}

func (s *completeStep) Run(ctx context.Context) error {
	rs := s.rs
	rs.Tracker.UpdatePhase(progress.PhaseComplete, "active", 0, "Review completed")

	if rs.FinalReport != nil {
		finalReportData := dsession.FinalReportData{
			Verdict:    rs.FinalReport.Verdict,
			Summary:    rs.FinalReport.Summary,
			Score:      rs.FinalReport.Score,
			GatePassed: rs.FinalReport.GatePassed,
			GateReason: rs.FinalReport.GateReason,
		}
		if err := s.sessionService.CompleteSession(ctx, rs.SessionID, finalReportData); err != nil {
			logx.Errorf("complete session failed: %v", err)
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

// toWorkflowQuestions 把 requirements.ClarifyingQuestion 转为 workflow 本地类型。
func toWorkflowQuestions(qs []requirements.ClarifyingQuestion) []ClarifyingQuestion {
	if len(qs) == 0 {
		return nil
	}
	out := make([]ClarifyingQuestion, len(qs))
	for i, q := range qs {
		out[i] = ClarifyingQuestion{Category: q.Category, Question: q.Question, Context: q.Context}
	}
	return out
}

// toFindingDataSlice 把 agent.Finding 转为 dsession.FindingData（持久化到 session）。
func toFindingDataSlice(findings []agent.Finding) []dsession.FindingData {
	if len(findings) == 0 {
		return nil
	}
	out := make([]dsession.FindingData, len(findings))
	for i, f := range findings {
		out[i] = dsession.FindingData{
			ID: f.ID, Severity: f.Severity, Category: f.Category,
			Path: f.Path, Line: f.Line, EndLine: f.EndLine,
			Title: f.Title, Body: f.Body, Suggestion: f.Suggestion,
			Confidence: f.Confidence, Agents: f.Agents,
		}
	}
	return out
}

// generateFinalReport 生成最终报告
func generateFinalReport(
	aggregatedResult agent.AggregatedResult,
	discussionResult *discussion.DiscussionResult,
	requirementsCoverage *requirements.CoverageAssessment,
	changedLines int,
) FinalReport {
	var blockers []agent.Finding
	var shouldFix []agent.Finding
	var suggestions []agent.Finding
	counts := scoring.Counts{}

	// 单次遍历：同一 normalized severity 同时决定 bucket 归属和评分计数，
	// 确保「严重」问题不会被分到 suggestions 却按 critical 计分等不一致。
	for _, finding := range aggregatedResult.Findings {
		switch types.NormalizeSeverity(finding.Severity) {
		case "critical":
			blockers = append(blockers, finding)
			counts.Critical++
		case "major":
			shouldFix = append(shouldFix, finding)
			counts.Major++
		case "minor":
			suggestions = append(suggestions, finding)
			counts.Minor++
		default: // info
			suggestions = append(suggestions, finding)
			counts.Info++
		}
	}

	score := scoring.Compute(counts, changedLines, 0, true)

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
		Score:       score.Value,
		GatePassed:  gatePassed,
		GateReason:  gateReason,
	}
}

// sessionService 描述 workflow 所需的 session 存储契约。
//
// PR-11 修复：从匿名本地接口升级为带文档的命名类型，*session.SessionService
// 自然满足此接口（Go 隐式接口），无需在 session 包内显式声明。
// 测试可通过 mock 实现该接口。
type sessionService interface {
	CreateSession(ctx context.Context, reviewID string, round int) (*dsession.Session, error)
	GetNextRound(ctx context.Context, reviewID string) (int, error)
	UpdatePhase(ctx context.Context, sessionID string, phase string, progress float64) error
	UpdateAgentResults(ctx context.Context, sessionID string, agentResults []dsession.AgentResultData) error
	UpdateDiscussion(ctx context.Context, sessionID string, discussion dsession.DiscussionData) error
	UpdateFinalReport(ctx context.Context, sessionID string, finalReport dsession.FinalReportData) error
	CompleteSession(ctx context.Context, sessionID string, finalReport dsession.FinalReportData) error
	FailSession(ctx context.Context, sessionID string, reason string) error
}

// Compile-time assertion: *session.SessionService 必须实现 sessionService。
// 未来 session.Service 增加方法而忘了更新本接口时，编译会失败。
var _ sessionService = (*session.SessionService)(nil)
