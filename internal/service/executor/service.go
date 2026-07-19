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

	"github.com/yi-nology/code-review-service/internal/domain/projectpolicy"
	bizmetrics "github.com/yi-nology/code-review-service/internal/observability/metrics"
	"github.com/yi-nology/code-review-service/internal/ocr"
	"github.com/yi-nology/code-review-service/internal/repository"
	"github.com/yi-nology/code-review-service/internal/repository/repoerr"
	"github.com/yi-nology/code-review-service/internal/service/agent"
	"github.com/yi-nology/code-review-service/internal/service/cost"
	"github.com/yi-nology/code-review-service/internal/service/policy"
	"github.com/yi-nology/code-review-service/internal/service/progress"
	"github.com/yi-nology/code-review-service/internal/service/review"
	"github.com/yi-nology/code-review-service/internal/service/review/domain"
	"github.com/yi-nology/code-review-service/internal/service/statuscheck"
	"github.com/yi-nology/code-review-service/internal/service/types"
	"github.com/yi-nology/code-review-service/internal/service/workflow"
	"github.com/yi-nology/code-review-service/internal/vcs"

	notifdomain "github.com/yi-nology/code-review-service/internal/domain/notification"
	"github.com/oekit/concurrency/async"

	"github.com/oekit/logx"
	"github.com/oekit/pkg/slicex"
)

type Service struct {
	reviewRepo      repository.ReviewRepository
	projectRepo     repository.ProjectPolicyRepository
	globalRulesRepo repository.GlobalRulesRepository
	rulePackageRepo repository.OCRRulePackageRepository
	agentRepo       repository.ReviewerAgentRepository
	ocrSettingsRepo repository.OCRSettingsRepository
	runner          domain.Runner
	writeback       domain.WritebackPublisher
	notifier        domain.Notifier
	statusPoster    *statuscheck.Poster
	modelNameFunc   func() string
	budgetChecker   *cost.BudgetChecker
	costCalc        *cost.Calculator
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
	AgentRepo       repository.ReviewerAgentRepository
	OCRSettingsRepo repository.OCRSettingsRepository
	Runner          domain.Runner
	Writeback       domain.WritebackPublisher
	Notifier        domain.Notifier
	StatusPoster    *statuscheck.Poster
	// ModelNameFunc 延迟返回当前配置的模型名（用于成本归因）。可选，nil 时留空。
	ModelNameFunc func() string
	BudgetChecker *cost.BudgetChecker
	CostCalc      *cost.Calculator
	Policy        *policy.Engine
	ProgressMgr   *progress.ProgressManager
	Workflow      *workflow.ReviewWorkflow
	AgentManager  *agent.AgentManager
	LLMMetrics    *ocr.Metrics
}

func NewService(cfg Config) *Service {
	return &Service{
		reviewRepo:      cfg.ReviewRepo,
		projectRepo:     cfg.ProjectRepo,
		globalRulesRepo: cfg.GlobalRulesRepo,
		rulePackageRepo: cfg.RulePackageRepo,
		agentRepo:       cfg.AgentRepo,
		ocrSettingsRepo: cfg.OCRSettingsRepo,
		runner:          cfg.Runner,
		writeback:       cfg.Writeback,
		notifier:        cfg.Notifier,
		statusPoster:    cfg.StatusPoster,
		modelNameFunc:   cfg.ModelNameFunc,
		budgetChecker:   cfg.BudgetChecker,
		costCalc:        cfg.CostCalc,
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

	// 预算闸门：月度成本预算超限时，block 模式拒绝新评审（避免持续烧钱）。
	if s.budgetChecker != nil {
		if err := s.budgetChecker.CheckBlock(ctx); err != nil {
			s.recordFailure(ctx, id, req, "budget", err)
			return domain.Response{}, err
		}
	}

	if progressReporter != nil {
		ctx = progress.WithProgressReporter(ctx, progressReporter)
	}

	initialInput := domain.OCRInput{
		Binary: review.ResolveOCRBinary(ctx, s.ocrSettingsRepo),
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

	projectPolicy := projectpolicy.NewDefault(req.Provider, req.Repo)
	if s.projectRepo != nil {
		// webhook 回调审查的项目若未配置策略，自动创建默认记录落库，使其出现在项目策略页可被治理。
		if p, err := s.projectRepo.GetOrCreateDefault(ctx, req.Provider, req.Repo); err == nil {
			projectPolicy = p
		} else if !errors.Is(err, repoerr.ErrNotFound) {
			logx.Warn("failed to get/create project policy, using default", "error", err)
		}
	}
	workspace.OCR = review.ProjectPolicyToOCROverrides(projectPolicy)
	// 把需求/spec 注入 OCR --background：单 runner 路径直接调 OCR，此前丢失了需求上下文。
	// workflow 路径在 steps.go 里把需求嵌入 prompt，但 runner 路径只有 --background 这一个通道。
	if len(req.Requirements) > 0 {
		workspace.OCR.Background = review.MergeRequirementsIntoBackground(workspace.OCR.Background, req.Requirements)
	}

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
		rulePath, cleanup, ruleErr := review.ResolveRuleFileForAgents(ctx, projectPolicy, s.rulePackageRepo, s.globalRulesRepo, review.AgentRulePackageRefs(ctx, s.agentRepo)...)
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

	ocrInput := s.buildOCRInput(ctx, workspace, req)
	if err := s.updateWithOCRInput(ctx, id, ocrInput); err != nil {
		logx.Warn("failed to update OCR input", "review_id", id, "error", err)
	}

	// OCR v1.7.6+ resume：retry 同一 review 时，把上次 OCR 产出的 session_id 注入 workspace，
	// 让 runner 传 `--resume <id>` 复用已完成文件的 checkpoint。
	// 仅 SHA-mode（req.PullRequestID/req.MergeRequestIID 都为空）下生效——PR 模式的 HeadSHA
	// 可能是合成提交（restoreDeletedFiles / ApplyProjectScope），跨 retry 不稳定，resume 命中率不确定。
	// 调用方仍可通过 BuildConfig 层的 OCRDisableResume 关闭（防御 OCR session 不一致场景）。
	if req.PullRequestID == "" && req.MergeRequestIID == "" {
		if prior, err := s.reviewRepo.GetByReviewID(ctx, id); err == nil && prior != nil {
			if sid := strings.TrimSpace(prior.Response.Report.OCR.SessionID); sid != "" {
				workspace.ResumeSessionID = sid
				logx.Infof("[executor] review %s 启用 OCR resume，session_id=%s", id, sid)
			}
		}
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
		RulePath:     workspace.RulePath,
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
	// 统一计算跨 review 稳定指纹（与 runner 路径一致），供反馈关联与误报抑制。
	types.ComputeFingerprints(findings)

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
		Summary:          result.FinalReport.Summary,
		Findings:         findings,
		Scope:            scope,
		TechLeadAnalysis: tla,
		OCR: domain.OCRMetadata{
			Engine:        "workflow",
			Model:         s.currentModel(),
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

	s.notifyCompletion(req, resp)
	s.postCompletionStatus(req, resp, workspace)
	s.recordReviewMetrics(req, resp, workspace)
	return resp, nil
}

func (s *Service) executeWithRunner(ctx context.Context, id string, req domain.Request, workspace domain.Workspace, projectPolicy projectpolicy.ProjectPolicy) (domain.Response, error) {
	s.appendHistory(ctx, id, "ocr", "running", "code review model invoked", nil)
	// #7：runner 路径也计入 ocr.Metrics，让 /debug/llm/metrics 反映真实 OCR 调用（原仅 workflow 路径统计）。
	// 对齐 MetricsProvider.Complete 语义：墙钟 latency 无论成败都记录，调用前取 start。
	start := time.Now()
	result, err := s.runner.Run(ctx, workspace)
	if err != nil {
		if s.llmMetrics != nil {
			s.llmMetrics.TotalLatency.Add(int64(time.Since(start)))
			s.llmMetrics.Calls.Add(1)
			s.llmMetrics.Failures.Add(1)
		}
		// 把 runner 返回的 partial OCR metadata（含 session_id）持久化，供下次 retry --resume。
		s.recordFailureWithOCR(ctx, id, req, "ocr", err, result.OCR)
		return domain.Response{}, err
	}
	if s.llmMetrics != nil {
		s.llmMetrics.TotalLatency.Add(int64(time.Since(start)))
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

	// 在 gate/report/writeback 消费 findings 之前，统一计算跨 review 稳定指纹，
	// 供反馈关联、生命周期判定与误报抑制使用。
	types.ComputeFingerprints(result.Findings)

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

	s.notifyCompletion(req, resp)
	s.postCompletionStatus(req, resp, workspace)
	s.recordReviewMetrics(req, resp, workspace)
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
	// 回写指标：published / failed 分别计数。
	if wbResult.Published > 0 {
		bizmetrics.WritebackFindingsTotal.WithLabelValues(req.Provider, "published").Add(float64(wbResult.Published))
	}
	if wbResult.Failed > 0 {
		bizmetrics.WritebackFindingsTotal.WithLabelValues(req.Provider, "failed").Add(float64(wbResult.Failed))
	}
	return summary, nil
}

// recordFailure 记录评审失败：先追加一条 stage 级别的 failed history（带原始错误），
// 再持久化最终 failed 状态。stage 标识失败发生在哪个环节（repository/scope/ocr/workflow），
// 便于前端 timeline 与排障直接定位根因，而非只看到扁平的 "review execution failed"。
func (s *Service) recordFailure(ctx context.Context, id string, req domain.Request, stage string, runErr error) {
	s.recordFailureWithOCR(ctx, id, req, stage, runErr, domain.OCRMetadata{})
}

// recordFailureWithOCR 是 recordFailure 的扩展版，可携带 OCR 失败前的 partial metadata。
//
// 用于 OCR v1.7.6+ resume 支持：runner 在 OCR 子进程崩溃/超时时仍可能从 stdout 解析出
// 部分 JSON（含 session_id）。把它写入 response.report.ocr.session_id，
// 下次 retry 同一 review 时 executor 会读出来传 `--resume <id>`，复用已完成文件的 checkpoint。
func (s *Service) recordFailureWithOCR(ctx context.Context, id string, req domain.Request, stage string, runErr error, ocrMeta domain.OCRMetadata) {
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
	// 携带 partial OCR metadata（如 session_id），即使 review 整体失败也保留，
	// 供 retry 路径读出 session_id 传 --resume。
	if ocrMeta.SessionID != "" {
		ocrMeta.Engine = "open-code-review"
		ocrMeta.Status = "failed"
		response.Report.OCR = ocrMeta
	}
	if err := s.updateWithRawOCR(ctx, id, req, response, nil); err != nil {
		logx.Error("failed to persist review failure", "review_id", id, "error", err)
	}

	// 失败通知（fire-and-forget，不阻塞主流程，不因通知失败影响评审落库）。
	s.notifyAsync(notifdomain.Event{
		ReviewID: id,
		Provider: req.Provider,
		Repo:     req.Repo,
		Ref:      req.PullRequestID + req.MergeRequestIID,
		Kind:     notifdomain.KindFailed,
		Stage:    stage,
		Error:    runErr.Error(),
	})

	// 失败时向 Git 平台写 error 状态（PR 上显示评审失败的 check）。
	// SHA 用 req.HeadSHA（recordFailure 无 workspace 上下文）。
	if s.statusPoster != nil && req.HeadSHA != "" {
		s.statusPoster.Post(context.Background(), req.Provider, req.Repo, req.HeadSHA,
			statuscheck.StateError,
			"评审执行失败："+truncateForStatus(stage+": "+runErr.Error()),
			id)
	}
	// 失败评审判定计入指标。
	bizmetrics.ReviewTotal.WithLabelValues(req.Provider, "failed").Inc()
}

// postCompletionStatus 在评审结束后向 Git 平台写 commit status（CI 质量门禁）。
// gate passed → success，gate failed → failed，用 workspace 真实 head SHA。
func (s *Service) postCompletionStatus(req domain.Request, resp domain.Response, workspace domain.Workspace) {
	if s.statusPoster == nil {
		return
	}
	state := statuscheck.StateSuccess
	description := "代码评审通过"
	if resp.Gate.Status == domain.GateFailed {
		state = statuscheck.StateFailed
		description = "质量门禁未通过：" + truncateForStatus(resp.Gate.Reason)
	}
	// 用真实 head SHA（合成提交会让 workspace.HeadSHA 偏离）。
	sha := workspace.OriginalHeadSHA
	if sha == "" {
		sha = workspace.HeadSHA
	}
	if sha == "" {
		sha = req.HeadSHA
	}
	s.statusPoster.Post(context.Background(), req.Provider, req.Repo, sha, state, description, resp.ReviewID)
}

func truncateForStatus(s string) string {
	if s == "" {
		return "未知原因"
	}
	if len(s) > 100 {
		return s[:99] + "…"
	}
	return s
}

// currentModel 返回当前配置的模型名（用于成本归因）。
func (s *Service) currentModel() string {
	if s.modelNameFunc != nil {
		return s.modelNameFunc()
	}
	return ""
}

// recordReviewMetrics 记录评审完成的 Prometheus 指标：
// 耗时分布、评审计数、LLM 估算成本。
func (s *Service) recordReviewMetrics(req domain.Request, resp domain.Response, workspace domain.Workspace) {
	status := resp.Gate.Status
	if status == "" {
		status = resp.Status
	}
	bizmetrics.ReviewTotal.WithLabelValues(req.Provider, status).Inc()

	// 耗时：从 report.ocr.elapsed 解析（OCR 子进程耗时），fallback 用 total_tokens 推断。
	// 这里用 elapsed 字符串（如 "12.3s"）解析秒数。
	if elapsed := parseElapsedSeconds(resp.Report.OCR.Elapsed); elapsed > 0 {
		bizmetrics.ReviewDurationSeconds.WithLabelValues(req.Provider, status).Observe(elapsed)
	}

	// LLM 成本：用 token 用量 × 单价估算（cost.Calculator 可能未配置，nil 安全）。
	if s.costCalc != nil && resp.Report.OCR.TotalTokens > 0 {
		model := resp.Report.OCR.Model
		cost := s.costCalc.Estimate(context.Background(), req.Provider, model,
			resp.Report.OCR.InputTokens, resp.Report.OCR.OutputTokens,
			resp.Report.OCR.CacheReadTokens, resp.Report.OCR.CacheWriteTokens)
		if cost > 0 {
			bizmetrics.LLMCostUSDTotal.WithLabelValues(req.Provider, model).Add(cost)
		}
	}
}

// parseElapsedSeconds 解析 OCR elapsed 字符串（如 "12.3s"）为秒数。
func parseElapsedSeconds(elapsed string) float64 {
	if elapsed == "" {
		return 0
	}
	// 去掉单位后缀 s/ms，尝试解析。
	s := strings.TrimSuffix(elapsed, "s")
	var v float64
	if _, err := fmt.Sscanf(s, "%f", &v); err != nil {
		return 0
	}
	// ms 单位
	if strings.HasSuffix(elapsed, "ms") {
		return v / 1000
	}
	return v
}

// notifyCompletion 在评审成功完成后触发通知。
// 根据 gate 结果区分 completed / gate_failed 事件。
func (s *Service) notifyCompletion(req domain.Request, resp domain.Response) {
	kind := notifdomain.KindCompleted
	if resp.Gate.Status == domain.GateFailed {
		kind = notifdomain.KindGateFailed
	}
	s.notifyAsync(notifdomain.Event{
		ReviewID:   resp.ReviewID,
		Provider:   req.Provider,
		Repo:       req.Repo,
		Ref:        req.PullRequestID + req.MergeRequestIID,
		Kind:       kind,
		GateStatus: resp.Gate.Status,
		Score:      resp.Report.Score.Value,
		Grade:      resp.Report.Score.Grade,
		Findings:   len(resp.Findings),
		Summary:    resp.Summary,
	})
}

// notifyAsync 异步触发通知（fire-and-forget）。
// 用独立 context 避免评审 ctx 取消后通知也跟着取消；通知失败仅记日志。
func (s *Service) notifyAsync(event notifdomain.Event) {
	if s.notifier == nil {
		return
	}
	async.GoSafe(func() {
		notifyCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := s.notifier.Notify(notifyCtx, event); err != nil {
			logx.CtxWarnf(context.Background(),
				"[notification] review %s kind=%s notify failed: %v", event.ReviewID, event.Kind, err)
		}
	})
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

func (s *Service) buildOCRInput(ctx context.Context, workspace domain.Workspace, req domain.Request) domain.OCRInput {
	return domain.OCRInput{
		Binary: review.ResolveOCRBinary(ctx, s.ocrSettingsRepo),
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
