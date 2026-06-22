package executor

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/yi-nology/code-review-service/internal/repository"
	"github.com/yi-nology/code-review-service/internal/repository/postgres/models"
	"github.com/yi-nology/code-review-service/internal/service/agent"
	"github.com/yi-nology/code-review-service/internal/service/policy"
	"github.com/yi-nology/code-review-service/internal/service/progress"
	"github.com/yi-nology/code-review-service/internal/service/review"
	"github.com/yi-nology/code-review-service/internal/service/reviewmap"
	"github.com/yi-nology/code-review-service/internal/service/runner"
	"github.com/yi-nology/code-review-service/internal/service/workflow"

	"github.com/oekit/pkg/slicex"
	"github.com/cloudwego/hertz/pkg/common/hlog"
)

type Service struct {
	reviewRepo      repository.ReviewRepository
	projectRepo     repository.ProjectPolicyRepository
	globalRulesRepo repository.GlobalRulesRepository
	reviewMapRepo   repository.ReviewMapRepository
	runner          review.Runner
	writeback       review.WritebackPublisher
	policy          *policy.Engine
	progressMgr     *progress.ProgressManager
	workflow        *workflow.ReviewWorkflow
	agentManager    *agent.AgentManager
}

type Config struct {
	ReviewRepo      repository.ReviewRepository
	ProjectRepo     repository.ProjectPolicyRepository
	GlobalRulesRepo repository.GlobalRulesRepository
	ReviewMapRepo   repository.ReviewMapRepository
	Runner          review.Runner
	Writeback       review.WritebackPublisher
	Policy          *policy.Engine
	ProgressMgr     *progress.ProgressManager
	Workflow        *workflow.ReviewWorkflow
	AgentManager    *agent.AgentManager
}

func NewService(cfg Config) *Service {
	return &Service{
		reviewRepo:      cfg.ReviewRepo,
		projectRepo:     cfg.ProjectRepo,
		globalRulesRepo: cfg.GlobalRulesRepo,
		reviewMapRepo:   cfg.ReviewMapRepo,
		runner:          cfg.Runner,
		writeback:       cfg.Writeback,
		policy:          cfg.Policy,
		progressMgr:     cfg.ProgressMgr,
		workflow:        cfg.Workflow,
		agentManager:    cfg.AgentManager,
	}
}

func (s *Service) shouldUseWorkflow() bool {
	if s.workflow == nil || s.agentManager == nil {
		return false
	}
	agents := s.agentManager.GetAgents()
	return len(agents) > 1
}

func (s *Service) Execute(ctx context.Context, id string, req review.Request, provider review.Provider, progressReporter progress.ProgressReporter) (review.Response, error) {
	if err := s.updateStatus(ctx, id, "running"); err != nil {
		return review.Response{}, err
	}

	if progressReporter != nil {
		ctx = progress.WithProgressReporter(ctx, progressReporter)
	}

	initialInput := review.OCRInput{
		Binary: "ocr",
		Workspace: review.WorkspaceInfo{
			Provider: req.Provider,
			Repo:     req.Repo,
			BaseSHA:  req.BaseSHA,
			HeadSHA:  req.HeadSHA,
		},
		StartedAt: time.Now().UTC(),
	}
	if err := s.updateWithOCRInput(ctx, id, initialInput); err != nil {
		hlog.Warn("failed to save initial OCR input", "review_id", id, "error", err)
	}

	workspace, err := provider.Prepare(ctx, req)
	if err != nil {
		s.recordFailure(ctx, id, req, "repository", err)
		return review.Response{}, err
	}
	s.appendHistory(ctx, id, "repository", "completed", "repository and change scope prepared", map[string]any{
		"base_sha": workspace.BaseSHA,
		"head_sha": workspace.HeadSHA,
	})

	if workspace.BaseSHA != "" || workspace.HeadSHA != "" {
		req.BaseSHA = workspace.BaseSHA
		req.HeadSHA = workspace.HeadSHA
		s.updateRequestSHAs(ctx, id, req)
	}

	projectPolicy := models.ProjectPolicy{FailOn: "critical"}
	if s.projectRepo != nil {
		p, err := s.projectRepo.GetByProviderRepo(ctx, req.Provider, req.Repo)
		if err == nil && p != nil {
			projectPolicy = *p
		}
	}
	workspace.OCR = projectPolicyOCROverrides(projectPolicy)

	var ignorePaths []string
	if len(projectPolicy.IgnorePaths) > 0 {
		if err := json.Unmarshal(projectPolicy.IgnorePaths, &ignorePaths); err != nil {
			hlog.Warn("failed to decode ignore_paths", "error", err)
		}
	}
	if len(ignorePaths) > 0 {
		workspace, err = applyProjectScope(ctx, workspace, ignorePaths)
		if err != nil {
			s.recordFailure(ctx, id, req, "scope", err)
			return review.Response{}, err
		}
		s.appendHistory(ctx, id, "scope", "completed", "project review policy applied", map[string]any{
			"changed_files":  workspace.Scope.ChangedFiles,
			"reviewed_files": workspace.Scope.ReviewedFiles,
			"excluded_files": workspace.Scope.ExcludedFiles,
		})
	}

	if s.globalRulesRepo != nil {
		rulePath, cleanup, ruleErr := s.resolveRuleFile(ctx, projectPolicy)
		if ruleErr != nil {
			hlog.Warn("failed to resolve rule file", "error", ruleErr)
		} else if rulePath != "" {
			workspace.RulePath = rulePath
			if cleanup != nil {
				defer cleanup()
			}
		}
	}

	if workspace.Dir != "" {
		defer func() {
			if err := os.RemoveAll(workspace.Dir); err != nil {
				hlog.Warn("failed to remove review workspace", "dir", workspace.Dir, "error", err)
			}
		}()
	}

	ocrInput := buildOCRInput(workspace, req)
	if err := s.updateWithOCRInput(ctx, id, ocrInput); err != nil {
		hlog.Warn("failed to update OCR input", "review_id", id, "error", err)
	}

	if s.shouldUseWorkflow() {
		return s.executeWithWorkflow(ctx, id, req, workspace, projectPolicy)
	}

	return s.executeWithRunner(ctx, id, req, workspace, projectPolicy)
}

func (s *Service) executeWithWorkflow(ctx context.Context, id string, req review.Request, workspace review.Workspace, projectPolicy models.ProjectPolicy) (review.Response, error) {
	s.appendHistory(ctx, id, "workflow", "running", "multi-agent review workflow started", nil)

	wfReq := workflow.ReviewRequest{
		ReviewID:     id,
		Provider:     req.Provider,
		Repo:         req.Repo,
		BaseSHA:      workspace.BaseSHA,
		HeadSHA:      workspace.HeadSHA,
		Diff:         workspace.Diff,
		Dir:          workspace.Dir,
		Round:        1,
		Requirements: req.Requirements,
	}
	// 映射临时 reviewer：review.EphemeralReviewer → agent.EphemeralSpec（阶段 5）
	if len(req.EphemeralReviewers) > 0 {
		wfReq.EphemeralReviewers = make([]agent.EphemeralSpec, len(req.EphemeralReviewers))
		for i, e := range req.EphemeralReviewers {
			wfReq.EphemeralReviewers[i] = agent.EphemeralSpec{Name: e.Name, Background: e.Background}
		}
	}

	result, err := s.workflow.RunReview(ctx, wfReq)
	if err != nil {
		s.recordFailure(ctx, id, req, "workflow", err)
		return review.Response{}, err
	}

	s.appendHistory(ctx, id, "workflow", "completed", fmt.Sprintf("multi-agent review completed with %d findings", len(result.AgentResults)), nil)

	var findings []review.Finding
	for _, f := range result.FinalReport.Blockers {
		findings = append(findings, review.Finding{
			Severity: review.SeverityCritical,
			Path:     f.Path,
			Line:     f.Line,
			Title:    f.Title,
			Body:     f.Body,
		})
	}
	for _, f := range result.FinalReport.ShouldFix {
		findings = append(findings, review.Finding{
			Severity: review.SeverityMajor,
			Path:     f.Path,
			Line:     f.Line,
			Title:    f.Title,
			Body:     f.Body,
		})
	}
	for _, f := range result.FinalReport.Suggestions {
		findings = append(findings, review.Finding{
			Severity: review.SeverityMinor,
			Path:     f.Path,
			Line:     f.Line,
			Title:    f.Title,
			Body:     f.Body,
		})
	}

	scope := workspace.Scope
	scope.ChangedLines = review.CountDiffLines(workspace.Diff)

	dummyResult := review.Result{
		Summary:  result.FinalReport.Summary,
		Findings: findings,
		Scope:    scope,
		OCR: review.OCRMetadata{
			Engine:        "workflow",
			Status:        review.OCRStatusSuccess,
			FilesReviewed: int64(scope.ChangedFiles),
		},
	}
	report := review.BuildReport(&dummyResult)
	analysis := review.Analysis{
		Overview: result.FinalReport.Summary,
	}
	// B2：把 Tech Lead 分析映射进 review.Analysis（前端已读该字段）
	if result.TechLeadAnalysis != nil {
		if result.TechLeadAnalysis.Summary != "" {
			analysis.Overview = result.TechLeadAnalysis.Summary
		}
		analysis.Risks = result.TechLeadAnalysis.Risks
		analysis.Recommendations = result.TechLeadAnalysis.Recommendations
	}

	gateEngine := s.policy
	if projectPolicy.FailOn != "" {
		gateEngine = policy.NewEngine(policy.Config{FailOn: projectPolicy.FailOn})
	}
	gate := gateEngine.Evaluate(toPolicyFindings(findings))

	resp := review.Response{
		ReviewID: id,
		Status:   "success",
		Summary:  result.FinalReport.Summary,
		Report:   report,
		Analysis: analysis,
		Findings: findings,
		Gate:     review.CompletedGate(gate.Passed, gate.Reason),
	}

	if err := s.updateWithRawOCR(ctx, id, req, resp, nil); err != nil {
		return review.Response{}, err
	}

	return resp, nil
}

func (s *Service) executeWithRunner(ctx context.Context, id string, req review.Request, workspace review.Workspace, projectPolicy models.ProjectPolicy) (review.Response, error) {
	s.appendHistory(ctx, id, "ocr", "running", "code review model invoked", nil)
	result, err := s.runner.Run(ctx, workspace)
	if err != nil {
		s.recordFailure(ctx, id, req, "ocr", err)
		return review.Response{}, err
	}
	s.appendHistory(ctx, id, "ocr", "completed", "code review model analysis completed", map[string]any{
		"files_reviewed": result.OCR.FilesReviewed,
		"comments":       result.OCR.Comments,
		"total_tokens":   result.OCR.TotalTokens,
		"elapsed":        result.OCR.Elapsed,
	})
	result.Scope = workspace.Scope
	result.Scope.ChangedLines = review.CountDiffLines(workspace.Diff)

	report := review.BuildReport(&result)
	analysis := review.BuildAnalysis(result)
	gateEngine := s.policy
	if projectPolicy.FailOn != "" {
		gateEngine = policy.NewEngine(policy.Config{FailOn: projectPolicy.FailOn})
	}
	gate := gateEngine.Evaluate(toPolicyFindings(result.Findings))
	resp := review.Response{
		ReviewID: id,
		Status:   "success",
		Summary:  result.Summary,
		Report:   report,
		Analysis: analysis,
		Findings: result.Findings,
		Gate:     review.CompletedGate(gate.Passed, gate.Reason),
	}

	callbackMode := req.CallbackMode
	hasPR := strings.TrimSpace(req.PullRequestID) != "" || strings.TrimSpace(req.MergeRequestIID) != ""
	if !hasPR && (callbackMode == "" || callbackMode == "comment") {
		callbackMode = "none"
	}

	if callbackMode != "none" && callbackMode != "summary" && s.writeback != nil {
		writebackReq := req
		if workspace.OriginalHeadSHA != "" {
			writebackReq.HeadSHA = workspace.OriginalHeadSHA
		}
		if err := s.writeback.PublishWithDiff(ctx, writebackReq, result, workspace.Diff); err != nil {
			hlog.Warn("writeback publish failed", "error", err)
		}
	}

	if err := s.updateWithRawOCR(ctx, id, req, resp, result.RawOCR); err != nil {
		return review.Response{}, err
	}

	if s.reviewMapRepo != nil && workspace.Diff != "" {
		s.generateAndSaveReviewMap(ctx, id, req, workspace, result)
	}

	return resp, nil
}

// recordFailure 记录评审失败：先追加一条 stage 级别的 failed history（带原始错误），
// 再持久化最终 failed 状态。stage 标识失败发生在哪个环节（repository/scope/ocr/workflow），
// 便于前端 timeline 与排障直接定位根因，而非只看到扁平的 "review execution failed"。
func (s *Service) recordFailure(ctx context.Context, id string, req review.Request, stage string, runErr error) {
	details := map[string]any{
		"error": runErr.Error(),
		"stage": stage,
	}
	if et := classifyReviewErrorType(runErr); et != "" {
		details["error_type"] = et
	}
	s.appendHistory(ctx, id, stage, "failed", runErr.Error(), details)

	response := review.Response{
		ReviewID: id,
		Status:   "failed",
		Summary:  "review failed",
		Gate:     review.ErrorGate("review execution failed"),
		Error:    runErr.Error(),
	}
	if err := s.updateWithRawOCR(ctx, id, req, response, nil); err != nil {
		hlog.Error("failed to persist review failure", "review_id", id, "error", err)
	}
}

// classifyReviewErrorType 把 runner 的结构化错误归类为运维可读标签，写入 history details，
// 让前端/排障能一眼区分是「二进制缺失 / 超时 / 子进程退出（LLM/DNS/解析）」。
// 非 runner 路径的错误（repository/scope/workflow stage）返回空。
func classifyReviewErrorType(err error) string {
	if runner.IsBinaryMissing(err) {
		return "binary_missing"
	}
	if runner.IsTimeout(err) {
		return "timeout"
	}
	if runner.IsExit(err) {
		return "exit"
	}
	return ""
}

func (s *Service) generateAndSaveReviewMap(ctx context.Context, id string, req review.Request, workspace review.Workspace, result review.Result) {
	var findings []reviewmap.Finding
	for i, f := range result.Findings {
		findings = append(findings, reviewmap.Finding{
			ID:         fmt.Sprintf("finding-%d", i),
			Severity:   string(f.Severity),
			Category:   f.Category,
			Path:       f.Path,
			Line:       f.Line,
			EndLine:    f.EndLine,
			Title:      f.Title,
			Body:       f.Body,
			Suggestion: f.SuggestionCode,
			Confidence: 0.8,
			Agents:     []string{"default"},
		})
	}

	generator := reviewmap.NewGenerator()
	reviewMap, err := generator.Generate(id, workspace.Diff, findings)
	if err != nil {
		hlog.Warn("failed to generate review map", "review_id", id, "error", err)
		return
	}

	dbMap := &models.ReviewMap{
		MapID:          reviewMap.ID,
		ReviewID:       id,
		SessionID:      id,
		Title:          reviewMap.Title,
		Description:    reviewMap.Description,
		Sections:       review.MustMarshal(reviewMap.Sections),
		Dependencies:   review.MustMarshal(reviewMap.Dependencies),
		Summary:        reviewMap.Summary,
		FileCount:      reviewMap.FileCount,
		SectionCount:   reviewMap.SectionCount,
		MermaidDiagram: reviewMap.ToMermaid(),
	}

	if err := s.reviewMapRepo.Create(ctx, dbMap); err != nil {
		hlog.Warn("failed to save review map", "review_id", id, "error", err)
		return
	}

	hlog.Info("review map generated and saved", "review_id", id, "map_id", reviewMap.ID, "sections", reviewMap.SectionCount, "files", reviewMap.FileCount)
}

func toPolicyFindings(findings []review.Finding) []policy.Finding {
	return slicex.Map(findings, func(f review.Finding) policy.Finding {
		return policy.Finding{Severity: string(f.Severity), Path: f.Path}
	})
}

func projectPolicyOCROverrides(p models.ProjectPolicy) review.OCROverrides {
	return review.OCROverrides{
		Enabled:                 p.OCREnabled,
		Concurrency:             p.OCRConcurrency,
		TimeoutMinutes:          p.OCRTimeoutMinutes,
		MaxTools:                p.OCRMaxTools,
		DefaultSeverity:         p.OCRDefaultSeverity,
		Background:              p.OCRBackground,
		Audience:                p.OCRAudience,
		Provider:                p.OCRProvider,
		URL:                     p.OCRURL,
		Model:                   p.OCRModel,
		MaxGitProcs:             p.OCRMaxGitProcs,
		UseAnthropic:            p.OCRUseAnthropic,
		ExtraBody:               p.OCRExtraBody,
		Language:                p.OCRLanguage,
		Binary:                  p.OCRBinary,
		ToolsPath:               p.OCRToolsPath,
		TelemetryEnabled:        p.OCRTelemetryEnabled,
		TelemetryExporter:       p.OCRTelemetryExporter,
		TelemetryOTLPEndpoint:   p.OCRTelemetryOTLPEndpoint,
		TelemetryContentLogging: p.OCRTelemetryContentLogging,
	}
}

func buildOCRInput(workspace review.Workspace, req review.Request) review.OCRInput {
	return review.OCRInput{
		Binary: "ocr",
		Args: []string{
			"review",
			"--repo", workspace.Dir,
			"--from", workspace.BaseSHA,
			"--to", workspace.HeadSHA,
			"--format", "json",
		},
		Workspace: review.WorkspaceInfo{
			Provider: req.Provider,
			Repo:     req.Repo,
			BaseSHA:  workspace.BaseSHA,
			HeadSHA:  workspace.HeadSHA,
			Dir:      workspace.Dir,
		},
		RulePath:  workspace.RulePath,
		StartedAt: time.Now().UTC(),
	}
}
