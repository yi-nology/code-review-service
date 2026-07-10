package executor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/yi-nology/code-review-service/internal/ocr"
	"github.com/yi-nology/code-review-service/internal/repository"
	"github.com/yi-nology/code-review-service/internal/domain/projectpolicy"
	"github.com/yi-nology/code-review-service/internal/repository/repoerr"
	"github.com/yi-nology/code-review-service/internal/service/agent"
	"github.com/yi-nology/code-review-service/internal/service/policy"
	"github.com/yi-nology/code-review-service/internal/service/progress"
	"github.com/yi-nology/code-review-service/internal/service/review"
	"github.com/yi-nology/code-review-service/internal/service/review/domain"
	"github.com/yi-nology/code-review-service/internal/service/workflow"
	"github.com/yi-nology/code-review-service/internal/vcs"

	"github.com/oekit/logx"
	"github.com/oekit/pkg/slicex"
)

type Service struct {
	reviewRepo      repository.ReviewRepository
	projectRepo     repository.ProjectPolicyRepository
	globalRulesRepo repository.GlobalRulesRepository
	rulePackageRepo repository.OCRRulePackageRepository
	runner          domain.Runner
	writeback       domain.WritebackPublisher
	policy          *policy.Engine
	progressMgr     *progress.ProgressManager
	workflow        *workflow.ReviewWorkflow
	agentManager    *agent.AgentManager
	llmMetrics      *ocr.Metrics

	// running tracks currently executing reviews for cancellation support
	runningMu sync.Mutex
	running   map[string]context.CancelFunc
}

type Config struct {
	ReviewRepo      repository.ReviewRepository
	ProjectRepo     repository.ProjectPolicyRepository
	GlobalRulesRepo repository.GlobalRulesRepository
	RulePackageRepo repository.OCRRulePackageRepository
	Runner          domain.Runner
	Writeback       domain.WritebackPublisher
	Policy          *policy.Engine
	ProgressMgr     *progress.ProgressManager
	Workflow        *workflow.ReviewWorkflow
	AgentManager    *agent.AgentManager
	LLMMetrics      *ocr.Metrics
}

func NewService(cfg Config) *Service {
	return &Service{
		reviewRepo:      cfg.ReviewRepo,
		projectRepo:     cfg.ProjectRepo,
		globalRulesRepo: cfg.GlobalRulesRepo,
		rulePackageRepo: cfg.RulePackageRepo,
		runner:          cfg.Runner,
		writeback:       cfg.Writeback,
		policy:          cfg.Policy,
		progressMgr:     cfg.ProgressMgr,
		workflow:        cfg.Workflow,
		agentManager:    cfg.AgentManager,
		llmMetrics:      cfg.LLMMetrics,
		running:         make(map[string]context.CancelFunc),
	}
}

func (s *Service) shouldUseWorkflow() bool {
	if s.workflow == nil || s.agentManager == nil {
		return false
	}
	agents := s.agentManager.GetAgents()
	return len(agents) > 1
}

// CancelExecution cancels a running execution for the given review ID
func (s *Service) CancelExecution(reviewID string) {
	s.runningMu.Lock()
	if cancel, ok := s.running[reviewID]; ok {
		logx.Infof("cancelling execution for review %s", reviewID)
		cancel()
		delete(s.running, reviewID)
	}
	s.runningMu.Unlock()
}

func (s *Service) Execute(ctx context.Context, id string, req domain.Request, provider domain.Provider, progressReporter progress.ProgressReporter) (domain.Response, error) {
	// Cancel any existing execution for this review
	s.CancelExecution(id)

	// Create a cancellable context for this execution
	ctx, cancel := context.WithCancel(ctx)
	s.runningMu.Lock()
	s.running[id] = cancel
	s.runningMu.Unlock()

	// Ensure we clean up when done
	defer func() {
		s.runningMu.Lock()
		delete(s.running, id)
		s.runningMu.Unlock()
	}()

	if err := s.updateStatus(ctx, id, "running"); err != nil {
		return domain.Response{}, err
	}

	if progressReporter != nil {
		ctx = progress.WithProgressReporter(ctx, progressReporter)
	}

	initialInput := domain.OCRInput{
		Binary: "ocr",
		Workspace: domain.WorkspaceInfo{
			Provider: req.Provider,
			Repo:     req.Repo,
			BaseSHA:  req.BaseSHA,
			HeadSHA:  req.HeadSHA,
		},
		StartedAt: time.Now().UTC(),
	}
	if err := s.updateWithOCRInput(ctx, id, initialInput); err != nil {
		logx.Warn("failed to save initial OCR input", "review_id", id, "error", err)
	}

	workspace, err := provider.Prepare(ctx, req)
	if err != nil {
		s.recordFailure(ctx, id, req, "repository", err)
		return domain.Response{}, err
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

	projectPolicy := projectpolicy.ProjectPolicy{FailOn: "critical"}
	if s.projectRepo != nil {
		p, err := s.projectRepo.GetByProviderRepo(ctx, req.Provider, req.Repo)
		if err != nil && !errors.Is(err, repoerr.ErrNotFound) {
			logx.Warn("failed to get project policy, using default", "error", err)
		}
		if p != nil {
			projectPolicy = *p
		}
	}
	workspace.OCR = review.ProjectPolicyToOCROverrides(projectPolicy)

	var ignorePaths []string
	if len(projectPolicy.IgnorePaths) > 0 {
		if err := json.Unmarshal(projectPolicy.IgnorePaths, &ignorePaths); err != nil {
			logx.Warn("failed to decode ignore_paths", "error", err)
		}
	}
	if len(ignorePaths) > 0 {
		workspace, err = review.ApplyProjectScope(ctx, workspace, ignorePaths)
		if err != nil {
			s.recordFailure(ctx, id, req, "scope", err)
			return domain.Response{}, err
		}
		s.appendHistory(ctx, id, "scope", "completed", "project review policy applied", map[string]any{
			"changed_files":  workspace.Scope.ChangedFiles,
			"reviewed_files": workspace.Scope.ReviewedFiles,
			"excluded_files": workspace.Scope.ExcludedFiles,
		})
	}

	if s.globalRulesRepo != nil {
		rulePath, cleanup, ruleErr := review.ResolveRuleFile(ctx, projectPolicy, s.rulePackageRepo, s.globalRulesRepo)
		if ruleErr != nil {
			logx.Warn("failed to resolve rule file", "error", ruleErr)
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
				logx.Warn("failed to remove review workspace", "dir", workspace.Dir, "error", err)
			}
		}()
	}

	ocrInput := buildOCRInput(workspace, req)
	if err := s.updateWithOCRInput(ctx, id, ocrInput); err != nil {
		logx.Warn("failed to update OCR input", "review_id", id, "error", err)
	}

	// 如果 diff 为空但有 workspace 目录和 SHA，尝试从 git 计算 diff
	if strings.TrimSpace(workspace.Diff) == "" && workspace.Dir != "" && workspace.BaseSHA != "" && workspace.HeadSHA != "" {
		if git, gitErr := vcs.Open(workspace.Dir, ""); gitErr == nil {
			defer func() { _ = git.Close() }()
			if diff, diffErr := git.Diff(ctx, workspace.BaseSHA, workspace.HeadSHA); diffErr == nil && len(diff) > 0 {
				workspace.Diff = diff
				logx.Infof("recovered empty diff from git for review %s (%d bytes)", id, len(diff))
			} else {
				logx.Warnf("failed to recover diff from git for review %s: %v", id, diffErr)
			}
		}
	}

	if s.shouldUseWorkflow() {
		return s.executeWithWorkflow(ctx, id, req, workspace, projectPolicy)
	}

	return s.executeWithRunner(ctx, id, req, workspace, projectPolicy)
}

func (s *Service) executeWithWorkflow(ctx context.Context, id string, req domain.Request, workspace domain.Workspace, projectPolicy projectpolicy.ProjectPolicy) (domain.Response, error) {
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
		Language:     workspace.OCR.Language,
	}
	// 映射临时 reviewer：domain.EphemeralReviewer → agent.EphemeralSpec（阶段 5）
	if len(req.EphemeralReviewers) > 0 {
		wfReq.EphemeralReviewers = make([]agent.EphemeralSpec, len(req.EphemeralReviewers))
		for i, e := range req.EphemeralReviewers {
			wfReq.EphemeralReviewers[i] = agent.EphemeralSpec{Name: e.Name, Background: e.Background}
		}
	}

	result, err := s.workflow.RunReview(ctx, wfReq)
	if err != nil {
		s.recordFailure(ctx, id, req, "workflow", err)
		return domain.Response{}, err
	}

	s.appendHistory(ctx, id, "workflow", "completed", fmt.Sprintf("multi-agent review completed with %d findings", len(result.AgentResults)), nil)

	var findings []domain.Finding
	for _, bucket := range [][]agent.Finding{
		result.FinalReport.Blockers,
		result.FinalReport.ShouldFix,
		result.FinalReport.Suggestions,
	} {
		// agent.Finding 即 domain.Finding（types.Finding 别名），无需转换。
		findings = append(findings, bucket...)
	}

	// 确保 scope 被正确初始化（即使没有配置 ignorePaths）
	scope := workspace.Scope
	scope.ChangedLines = review.CountDiffLines(workspace.Diff)
	if scope.ChangedFiles == 0 {
		scope.ChangedFiles = review.CountDiffFiles(workspace.Diff)
	}
	if scope.ReviewedFiles == 0 {
		scope.ReviewedFiles = scope.ChangedFiles
	}

	// 汇总各 agent 的 token 使用（#21：原 workflow 路径 OCRMetadata 硬编码、token 全零，
	// 导致多 agent 评审在 token 成本统计里完全缺失）。
	var workflowTokens int64
	for _, ar := range result.AgentResults {
		workflowTokens += ar.TokensUsed
	}
	var tla *domain.TechLeadAnalysis
	if result.TechLeadAnalysis != nil {
		tla = &domain.TechLeadAnalysis{
			Summary:         result.TechLeadAnalysis.Summary,
			Risks:           result.TechLeadAnalysis.Risks,
			Recommendations: result.TechLeadAnalysis.Recommendations,
		}
	}
	dummyResult := domain.Result{
		Summary:           result.FinalReport.Summary,
		Findings:          findings,
		Scope:             scope,
		TechLeadAnalysis:  tla,
		OCR: domain.OCRMetadata{
			Engine:        "workflow",
			Status:        review.OCRStatusSuccess,
			FilesReviewed: int64(scope.ChangedFiles),
			Comments:      int64(len(findings)),
			TotalTokens:   workflowTokens,
			Elapsed:       result.Duration.Round(time.Millisecond).String(),
		},
	}
	report := review.BuildReport(&dummyResult)
	// workflow 路径：评分由 scoring.Compute 统一计算（BuildReport 和 generateFinalReport
	// 共用同一函数），两条路径分数天然一致，无需再覆盖。
	report.Complete = true
	report.Generated = true

	// PR-10 修复：复用 review.BuildAnalysis 推导 Analysis，与 runner 路径保持一致。
	// 之前手写 domain.Analysis{Overview: result.FinalReport.Summary} 会丢失
	// RootCause / Recommendations 等基于 findings 的字段，UI 上一直空着。
	analysis := review.BuildAnalysis(dummyResult)

	// workflow FinalReport 已含门禁结论（GatePassed/GateReason，对应 Verdict 如 REQUEST_CHANGES），
	// 直接采用以与最终报告同步；不再用 review-service policy 重算（会和 workflow 结论冲突）。
	gate := domain.CompletedGate(result.FinalReport.GatePassed, result.FinalReport.GateReason)

	resp := domain.Response{
		ReviewID: id,
		Status:   "success",
		Summary:  result.FinalReport.Summary,
		Report:   report,
		Analysis: analysis,
		Findings: findings,
		Gate:     gate,
	}

	wbSummary, wbErr := s.publishWriteback(ctx, id, req, workspace, domain.Result{Summary: resp.Summary, Findings: findings})
	resp.Writeback = wbSummary
	if wbErr != nil {
		resp.Status = domain.StatusSuccessWithPublishFailed
	}

	if err := s.updateWithRawOCR(ctx, id, req, resp, nil); err != nil {
		return resp, err
	}

	// writeback 失败不中断主流程，但把 error 透传给 caller 以便 metrics / 排障。
	if wbErr != nil {
		return resp, fmt.Errorf("review completed but writeback failed: %w", wbErr)
	}

	return resp, nil
}

func (s *Service) executeWithRunner(ctx context.Context, id string, req domain.Request, workspace domain.Workspace, projectPolicy projectpolicy.ProjectPolicy) (domain.Response, error) {
	s.appendHistory(ctx, id, "ocr", "running", "code review model invoked", nil)
	result, err := s.runner.Run(ctx, workspace)
	if err != nil {
		// #7：runner 路径也计入 ocr.Metrics，让 /debug/llm/metrics 反映真实 OCR 调用（原仅 workflow 路径统计）
		if s.llmMetrics != nil {
			s.llmMetrics.Calls.Add(1)
			s.llmMetrics.Failures.Add(1)
		}
		s.recordFailure(ctx, id, req, "ocr", err)
		return domain.Response{}, err
	}
	if s.llmMetrics != nil {
		s.llmMetrics.Calls.Add(1)
		s.llmMetrics.Successes.Add(1)
		s.llmMetrics.InputTokens.Add(result.OCR.InputTokens)
		s.llmMetrics.OutputTokens.Add(result.OCR.OutputTokens)
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
	resp := domain.Response{
		ReviewID: id,
		Status:   "success",
		Summary:  result.Summary,
		Report:   report,
		Analysis: analysis,
		Findings: result.Findings,
		Gate:     domain.CompletedGate(gate.Passed, gate.Reason),
	}

	wbSummary, wbErr := s.publishWriteback(ctx, id, req, workspace, domain.Result{Summary: result.Summary, Findings: result.Findings})
	resp.Writeback = wbSummary
	if wbErr != nil {
		resp.Status = domain.StatusSuccessWithPublishFailed
	}

	if err := s.updateWithRawOCR(ctx, id, req, resp, result.RawOCR); err != nil {
		if wbErr != nil {
			return resp, fmt.Errorf("review completed but writeback failed: %w", wbErr)
		}
		return domain.Response{}, err
	}

	if wbErr != nil {
		return resp, fmt.Errorf("review completed but writeback failed: %w", wbErr)
	}

	return resp, nil
}

// publishWriteback 在审查完成后将结果回写到代码平台（runner 和 workflow 路径共用）。
//
// 返回值：
//   - summary: 回写摘要，包含 status / published / failed / skipped / error
//     字段。调用方负责把 summary 落到 resp.Writeback，并根据 status 决定是否
//     把 resp.Status 改为 "success_with_publish_failed"。
//   - err: 仅当底层 writeback.PublishWithDiff 返回非 nil error 时非空。
//     history 上的 "writeback failed" 事件始终由本函数负责追加；不再静默吞错。
//
// 旧实现会把 error 写日志后吞掉，导致用户看到的 review 是 success 状态，
// 但实际评论没发出去——见 PR-01。
func (s *Service) publishWriteback(ctx context.Context, id string, req domain.Request, workspace domain.Workspace, result domain.Result) (domain.WritebackSummary, error) {
	if s.writeback == nil {
		return domain.WritebackSummary{Status: domain.WritebackStatusDisabled}, nil
	}

	callbackMode := req.CallbackMode
	hasPR := strings.TrimSpace(req.PullRequestID) != "" || strings.TrimSpace(req.MergeRequestIID) != ""
	if !hasPR && (callbackMode == "" || callbackMode == "comment") {
		callbackMode = "none"
	}

	if callbackMode == "none" || callbackMode == "summary" {
		return domain.WritebackSummary{Status: domain.WritebackStatusSkipped}, nil
	}

	s.appendHistory(ctx, id, "writeback", "running", "publishing review comments to git platform", nil)

	writebackReq := req
	writebackReq.ReviewID = id
	if workspace.OriginalHeadSHA != "" {
		writebackReq.HeadSHA = workspace.OriginalHeadSHA
	}
	wbResult, err := s.writeback.PublishWithDiff(ctx, writebackReq, result, workspace.Diff)
	if err != nil {
		logx.Warn("writeback publish failed", "review_id", id, "error", err)
		s.appendHistory(ctx, id, "writeback", "failed", fmt.Sprintf("writeback failed: %v", err), nil)
		return domain.WritebackSummary{
			Status: domain.WritebackStatusFailed,
			Error:  err.Error(),
		}, err
	}

	summary := domain.WritebackSummary{
		Status:    domain.WritebackStatusOK,
		Published: wbResult.Published,
		Failed:    wbResult.Failed,
		Skipped:   wbResult.Skipped,
	}
	switch {
	case wbResult.Stale:
		summary.Status = domain.WritebackStatusStale
		summary.Stale = true
		summary.ExpectedSHA = wbResult.ExpectedSHA
		summary.ActualSHA = wbResult.ActualSHA
		s.appendHistory(ctx, id, "writeback", "completed",
			fmt.Sprintf("writeback degraded (stale): head %.8s -> %.8s, summary-only published", wbResult.ExpectedSHA, wbResult.ActualSHA),
			map[string]any{"expected": wbResult.ExpectedSHA, "actual": wbResult.ActualSHA, "published": wbResult.Published})
	case wbResult.Skipped:
		summary.Status = domain.WritebackStatusSkipped
		s.appendHistory(ctx, id, "writeback", "completed", "writeback skipped (stale or empty)", nil)
	default:
		s.appendHistory(ctx, id, "writeback", "completed",
			fmt.Sprintf("comments published to git platform (published=%d, failed=%d)", wbResult.Published, wbResult.Failed),
			map[string]any{"published": wbResult.Published, "failed": wbResult.Failed})
	}
	return summary, nil
}

// recordFailure 记录评审失败：先追加一条 stage 级别的 failed history（带原始错误），
// 再持久化最终 failed 状态。stage 标识失败发生在哪个环节（repository/scope/ocr/workflow），
// 便于前端 timeline 与排障直接定位根因，而非只看到扁平的 "review execution failed"。
func (s *Service) recordFailure(ctx context.Context, id string, req domain.Request, stage string, runErr error) {
	details := map[string]any{
		"error": runErr.Error(),
		"stage": stage,
	}
	if et := classifyReviewErrorType(runErr); et != "" {
		details["error_type"] = et
	}
	s.appendHistory(ctx, id, stage, "failed", runErr.Error(), details)

	response := domain.Response{
		ReviewID: id,
		Status:   "failed",
		Summary:  "review failed",
		Gate:     domain.ErrorGate("review execution failed"),
		Error:    runErr.Error(),
	}
	if err := s.updateWithRawOCR(ctx, id, req, response, nil); err != nil {
		logx.Error("failed to persist review failure", "review_id", id, "error", err)
	}
}

// classifyReviewErrorType 把 runner 的结构化错误归类为运维可读标签，写入 history details，
// 让前端/排障能一眼区分是「二进制缺失 / 超时 / 子进程退出（LLM/DNS/解析）」。
// 非 runner 路径的错误（repository/scope/workflow stage）返回空。
func classifyReviewErrorType(err error) string {
	if ocr.IsBinaryMissing(err) {
		return "binary_missing"
	}
	if ocr.IsTimeout(err) {
		return "timeout"
	}
	if ocr.IsExit(err) {
		return "exit"
	}
	return ""
}

func toPolicyFindings(findings []domain.Finding) []policy.Finding {
	return slicex.Map(findings, func(f domain.Finding) policy.Finding {
		return policy.Finding{Severity: f.Severity, Path: f.Path}
	})
}

func buildOCRInput(workspace domain.Workspace, req domain.Request) domain.OCRInput {
	return domain.OCRInput{
		Binary: "ocr",
		Args: []string{
			"review",
			"--repo", workspace.Dir,
			"--from", workspace.BaseSHA,
			"--to", workspace.HeadSHA,
			"--format", "json",
		},
		Workspace: domain.WorkspaceInfo{
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
