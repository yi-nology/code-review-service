package eino

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	types "github.com/yi-nology/code-review-service/internal/domain/finding"
	"github.com/yi-nology/code-review-service/internal/domain/projectpolicy"
	"github.com/yi-nology/code-review-service/internal/ocr"
	"github.com/yi-nology/code-review-service/internal/repository"
	"github.com/yi-nology/code-review-service/internal/repository/repoerr"
	"github.com/yi-nology/code-review-service/internal/service/agent"
	"github.com/yi-nology/code-review-service/internal/service/cost"
	"github.com/yi-nology/code-review-service/internal/service/judge"
	"github.com/yi-nology/code-review-service/internal/service/policy"
	"github.com/yi-nology/code-review-service/internal/service/progress"
	"github.com/yi-nology/code-review-service/internal/service/review"
	"github.com/yi-nology/code-review-service/internal/service/review/domain"
	"github.com/yi-nology/code-review-service/internal/service/statuscheck"
	"github.com/yi-nology/code-review-service/internal/service/workflow"
	"github.com/yi-nology/code-review-service/internal/strutil"
	"github.com/yi-nology/code-review-service/internal/vcs"

	"github.com/oagentkit/obsx"
	"github.com/oekit/logx"
)

// Package eino：评审执行器核心——Execute 主流程与两条引擎路径（workflow/单 runner）。
// Executor implements the review.Executor interface using eino workflows.
// This replaces the old executor.Service with a cleaner, eino-based implementation.
type Executor struct {
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
	judge           *judge.Judge            // OCR 结果二次综合判断
	failureReporter *review.FailureReporter // 统一失败上报（历史/落库/通知/status/指标）

	// running tracks currently executing reviews for cancellation support
	runningMu sync.Mutex
	running   map[string]context.CancelFunc
}

// ExecutorConfig contains configuration for the eino executor
type ExecutorConfig struct {
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
	ModelNameFunc   func() string
	BudgetChecker   *cost.BudgetChecker
	CostCalc        *cost.Calculator
	Policy          *policy.Engine
	ProgressMgr     *progress.ProgressManager
	Workflow        *workflow.ReviewWorkflow
	AgentManager    *agent.AgentManager
	LLMMetrics      *ocr.Metrics
	Judge           *judge.Judge // OCR 结果二次综合判断（可 nil）
	// FailureReporter 统一失败上报；nil 时用 Notifier/StatusPoster 兜底构造
	//（保持旧行为），两者也缺时退化为仅落库。
	FailureReporter *review.FailureReporter
}

// NewExecutor creates a new eino-based executor
func NewExecutor(cfg ExecutorConfig) *Executor {
	failureReporter := cfg.FailureReporter
	if failureReporter == nil {
		failureReporter = review.NewFailureReporter(cfg.ReviewRepo, cfg.Notifier, cfg.StatusPoster)
	}
	return &Executor{
		reviewRepo:      cfg.ReviewRepo,
		projectRepo:     cfg.ProjectRepo,
		globalRulesRepo: cfg.GlobalRulesRepo,
		rulePackageRepo: cfg.RulePackageRepo,
		judge:           cfg.Judge,
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
		failureReporter: failureReporter,
		running:         make(map[string]context.CancelFunc),
	}
}

func (e *Executor) shouldUseWorkflow() bool {
	if e.workflow == nil || e.agentManager == nil {
		return false
	}
	agents := e.agentManager.GetAgents()
	return len(agents) > 1
}

// CancelExecution cancels a running execution for the given review ID
func (e *Executor) CancelExecution(reviewID string) {
	e.runningMu.Lock()
	if cancel, ok := e.running[reviewID]; ok {
		logx.Infof("cancelling execution for review %s", reviewID)
		cancel()
		delete(e.running, reviewID)
	}
	e.runningMu.Unlock()
}

// Execute implements the review.Executor interface.
// This is the main entry point for review execution.
func (e *Executor) Execute(ctx context.Context, id string, req domain.Request, provider domain.Provider, progressReporter progress.ProgressReporter) (domain.Response, error) {
	// Cancel any existing execution for this review
	e.CancelExecution(id)

	// Create a cancellable context for this execution
	ctx, cancel := context.WithCancel(ctx)
	e.runningMu.Lock()
	e.running[id] = cancel
	e.runningMu.Unlock()

	// 启用 eino LLM 调用追踪（结构化日志：stage/model/duration/tokens/慢调用告警）
	// OnUsage 回调将真实 token 用量注入 llmMetrics（覆盖 ReAct agent 直用 BaseChatModel 的场景）
	ctx = obsx.InitLLMObservability(ctx, logx.Default(), obsx.Options{
		SlowThreshold: 30 * time.Second,
		OnUsage: func(component, model, stage string, prompt, completion int) {
			if e.llmMetrics != nil {
				e.llmMetrics.InputTokens.Add(int64(prompt))
				e.llmMetrics.OutputTokens.Add(int64(completion))
			}
		},
	})

	// Ensure we clean up when done
	defer func() {
		e.runningMu.Lock()
		delete(e.running, id)
		e.runningMu.Unlock()
	}()

	if err := e.updateStatus(ctx, id, "running"); err != nil {
		return domain.Response{}, err
	}

	// === 评审开始：发 commit status + PR 评论 ===
	e.notifyStart(ctx, req)

	// Budget gate
	if e.budgetChecker != nil {
		if err := e.budgetChecker.CheckBlock(ctx); err != nil {
			e.recordFailure(ctx, id, req, "budget", err)
			return domain.Response{}, err
		}
	}

	if progressReporter != nil {
		ctx = progress.WithProgressReporter(ctx, progressReporter)
	}

	initialInput := domain.OCRInput{
		Binary: review.ResolveOCRBinary(ctx, e.ocrSettingsRepo),
		Workspace: domain.WorkspaceInfo{
			Provider: req.Provider,
			Repo:     req.Repo,
			BaseSHA:  req.BaseSHA,
			HeadSHA:  req.HeadSHA,
		},
		StartedAt: time.Now().UTC(),
	}
	if err := e.updateWithOCRInput(ctx, id, initialInput); err != nil {
		logx.Warn("failed to save initial OCR input", "review_id", id, "error", err)
	}

	// Step 1: 克隆仓库 + 准备工作区
	stepRepo := newAuditStep("repository")
	workspace, err := provider.Prepare(ctx, req)
	if err != nil {
		e.appendAuditStep(ctx, id, stepRepo.failed("仓库准备工作区失败", "git clone + resolve PR diff", req, err))
		e.recordFailure(ctx, id, req, "repository", err)
		return domain.Response{}, err
	}
	e.appendAuditStep(ctx, id, stepRepo.done(
		"仓库工作区准备完成",
		"git clone + resolve PR/MR diff",
		map[string]any{"provider": req.Provider, "repo": req.Repo, "base_sha": req.BaseSHA, "head_sha": req.HeadSHA, "pull_request_id": req.PullRequestID},
		map[string]any{"dir": workspace.Dir, "base_sha": workspace.BaseSHA, "head_sha": workspace.HeadSHA, "diff_bytes": len(workspace.Diff), "scope": workspace.Scope},
	))

	if workspace.BaseSHA != "" || workspace.HeadSHA != "" {
		req.BaseSHA = workspace.BaseSHA
		req.HeadSHA = workspace.HeadSHA
		e.updateRequestSHAs(ctx, id, req)
	}

	// Step 2: 加载项目策略
	stepPolicy := newAuditStep("policy")
	projectPolicy := projectpolicy.NewDefault(req.Provider, req.Repo)
	if e.projectRepo != nil {
		if p, err := e.projectRepo.GetOrCreateDefault(ctx, req.Provider, req.Repo); err == nil {
			projectPolicy = p
		} else if !errors.Is(err, repoerr.ErrNotFound) {
			logx.Warn("failed to get/create project policy, using default", "error", err)
		}
	}
	workspace.OCR = review.ProjectPolicyToOCROverrides(projectPolicy)
	if len(req.Requirements) > 0 {
		workspace.OCR.Background = review.MergeRequirementsIntoBackground(workspace.OCR.Background, req.Requirements)
	}

	// 记录项目策略配置
	var ignorePaths []string
	if len(projectPolicy.IgnorePaths) > 0 {
		if err := json.Unmarshal(projectPolicy.IgnorePaths, &ignorePaths); err != nil {
			logx.Warnf("[executor] failed to parse ignore_paths for %s/%s: %v", req.Provider, req.Repo, err)
		}
	}
	e.appendAuditStep(ctx, id, stepPolicy.done(
		"项目策略已加载",
		"从 DB 读取 project_policies 表，合并默认值",
		map[string]any{"provider": req.Provider, "repo": req.Repo},
		map[string]any{
			"fail_on":         projectPolicy.FailOn,
			"ignore_paths":    ignorePaths,
			"ocr_language":    workspace.OCR.Language,
			"ocr_background":  strutil.Truncate(workspace.OCR.Background, 200),
			"ocr_concurrency": workspace.OCR.Concurrency,
			"ocr_timeout":     workspace.OCR.TimeoutMinutes,
			"is_default":      projectPolicy.FailOn == "major" && len(ignorePaths) == 0,
		},
	))

	// Step 2.5: 过滤文件（如果配置了 ignore_paths）
	if len(ignorePaths) > 0 {
		stepScope := newAuditStep("scope")
		workspace, err = review.ApplyProjectScope(ctx, workspace, ignorePaths)
		if err != nil {
			e.appendAuditStep(ctx, id, stepScope.failed("应用项目审查策略失败", "apply ignore_paths filter", ignorePaths, err))
			e.recordFailure(ctx, id, req, "scope", err)
			return domain.Response{}, err
		}
		e.appendAuditStep(ctx, id, stepScope.done(
			"文件过滤完成",
			fmt.Sprintf("按 ignore_paths 过滤（%d 个规则）", len(ignorePaths)),
			map[string]any{"ignore_paths": ignorePaths, "diff_bytes_before": len(workspace.Diff)},
			map[string]any{"changed_files": workspace.Scope.ChangedFiles, "reviewed_files": workspace.Scope.ReviewedFiles, "excluded_files": workspace.Scope.ExcludedFiles},
		))
	}

	// Step 2.6: 合并规则
	stepRules := newAuditStep("rules")
	if e.globalRulesRepo != nil {
		rulePath, cleanup, ruleErr := review.ResolveRuleFileForAgents(ctx, projectPolicy, e.rulePackageRepo, e.globalRulesRepo, review.AgentRulePackageRefs(ctx, e.agentRepo)...)
		if ruleErr != nil {
			logx.Warn("failed to resolve rule file", "error", ruleErr)
			e.appendAuditStep(ctx, id, stepRules.failed("规则合并失败", "ResolveRuleFileForAgents", nil, ruleErr))
		} else if rulePath != "" {
			workspace.RulePath = rulePath
			if cleanup != nil {
				defer cleanup()
			}
			// 读取合并后的规则文件内容（截断显示）
			ruleContent := ""
			if data, readErr := os.ReadFile(rulePath); readErr == nil {
				ruleContent = strutil.Truncate(string(data), 1000)
			}
			e.appendAuditStep(ctx, id, stepRules.done(
				"审查规则已合并",
				"合并 6 层规则源：项目内联 → 项目规则包 → Agent 规则包 → 全局规则包 → 项目匹配规则包 → 旧版 global_rules",
				map[string]any{
					"project_rules":       strutil.Truncate(string(projectPolicy.Rules), 200),
					"rule_package_ids":    projectPolicy.RulePackageIDs,
					"agent_rule_packages": review.AgentRulePackageRefs(ctx, e.agentRepo),
				},
				map[string]any{"rule_path": rulePath, "rules_content": ruleContent},
			))
		} else {
			e.appendAuditStep(ctx, id, stepRules.done("无审查规则", "所有规则源均为空，LLM 自由审查", nil, nil))
		}
	} else {
		e.appendAuditStep(ctx, id, stepRules.done("规则仓库未配置", "globalRulesRepo 为 nil", nil, nil))
	}

	if workspace.Dir != "" {
		defer func() {
			if err := os.RemoveAll(workspace.Dir); err != nil {
				logx.Warn("failed to remove review workspace", "dir", workspace.Dir, "error", err)
			}
		}()
	}

	ocrInput := e.buildOCRInput(ctx, workspace, req)
	if err := e.updateWithOCRInput(ctx, id, ocrInput); err != nil {
		logx.Warn("failed to update OCR input", "review_id", id, "error", err)
	}

	// Step 2.7: Agent 选择
	stepAgent := newAuditStep("agent")
	if e.agentManager != nil {
		agents := e.agentManager.GetAgents()
		agentNames := make([]string, 0, len(agents))
		for _, a := range agents {
			agentNames = append(agentNames, a.Name())
		}
		execMode := "single-agent runner"
		if e.shouldUseWorkflow() {
			execMode = "multi-agent workflow"
		}
		e.appendAuditStep(ctx, id, stepAgent.done(
			fmt.Sprintf("Agent 选择完成：%d 个注册", len(agentNames)),
			"AgentManager.RunAgentsForProject → 4 层优先级：项目白名单 → 技术栈检测 → is_default → 全部",
			map[string]any{"provider": req.Provider, "repo": req.Repo},
			map[string]any{
				"registered_agents": agentNames,
				"total":             len(agentNames),
				"execution_mode":    execMode,
			},
		))
	} else {
		e.appendAuditStep(ctx, id, stepAgent.done("Agent 管理器未配置", "使用默认单 agent 模式", nil, nil))
	}

	// OCR resume support
	if !e.shouldUseWorkflow() {
		if prior, err := e.reviewRepo.GetByReviewID(ctx, id); err == nil && prior != nil {
			if sid := strings.TrimSpace(prior.Response.Report.OCR.SessionID); sid != "" {
				workspace.ResumeSessionID = sid
				logx.Infof("[executor] review %s 启用 OCR resume，session_id=%s", id, sid)
			}
		}
	}

	// Recover diff from git if empty
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

	// Step 2.8: OCR 执行参数
	stepOCRConfig := newAuditStep("ocr_config")
	e.appendAuditStep(ctx, id, stepOCRConfig.done(
		"OCR 执行参数已确定",
		"合并项目策略覆盖 + 全局 OCR 设置",
		map[string]any{
			"ocr_binary":     workspace.OCR.Binary,
			"ocr_language":   workspace.OCR.Language,
			"ocr_audience":   workspace.OCR.Audience,
			"ocr_provider":   workspace.OCR.Provider,
			"ocr_url":        workspace.OCR.URL,
			"ocr_model":      workspace.OCR.Model,
			"ocr_protocol":   workspace.OCR.Protocol,
			"ocr_background": strutil.Truncate(workspace.OCR.Background, 300),
			"concurrency":    workspace.OCR.Concurrency,
			"timeout_min":    workspace.OCR.TimeoutMinutes,
			"max_tools":      workspace.OCR.MaxTools,
			"rule_path":      workspace.RulePath,
			"diff_bytes":     len(workspace.Diff),
			"head_sha":       workspace.HeadSHA,
			"base_sha":       workspace.BaseSHA,
		},
		map[string]any{
			"ready":     workspace.Dir != "" && len(workspace.Diff) > 0,
			"dir":       workspace.Dir,
			"has_rules": workspace.RulePath != "",
		},
	))

	if e.shouldUseWorkflow() {
		return e.executeWithWorkflow(ctx, id, req, workspace, projectPolicy)
	}

	return e.executeWithRunner(ctx, id, req, workspace, projectPolicy)
}

func (e *Executor) executeWithWorkflow(ctx context.Context, id string, req domain.Request, workspace domain.Workspace, projectPolicy projectpolicy.ProjectPolicy) (domain.Response, error) {
	e.appendHistory(ctx, id, "workflow", "running", "multi-agent review workflow started", nil)

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
	if len(req.EphemeralReviewers) > 0 {
		wfReq.EphemeralReviewers = make([]agent.EphemeralSpec, len(req.EphemeralReviewers))
		for i, er := range req.EphemeralReviewers {
			wfReq.EphemeralReviewers[i] = agent.EphemeralSpec{Name: er.Name, Background: er.Background}
		}
	}

	result, err := e.workflow.RunReview(ctx, wfReq)
	if err != nil {
		e.recordFailure(ctx, id, req, "workflow", err)
		return domain.Response{}, err
	}

	e.appendHistory(ctx, id, "workflow", "completed", fmt.Sprintf("multi-agent review completed with %d findings", len(result.AgentResults)), nil)

	var findings []domain.Finding
	for _, bucket := range [][]agent.Finding{
		result.FinalReport.Blockers,
		result.FinalReport.ShouldFix,
		result.FinalReport.Suggestions,
	} {
		findings = append(findings, bucket...)
	}
	types.ComputeFingerprints(findings)

	scope := workspace.Scope
	scope.ChangedLines = review.CountDiffLines(workspace.Diff)
	if scope.ChangedFiles == 0 {
		scope.ChangedFiles = review.CountDiffFiles(workspace.Diff)
	}
	if scope.ReviewedFiles == 0 {
		scope.ReviewedFiles = scope.ChangedFiles
	}

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
			Model:         e.currentModel(),
			Status:        review.OCRStatusSuccess,
			FilesReviewed: int64(scope.ChangedFiles),
			Comments:      int64(len(findings)),
			TotalTokens:   workflowTokens,
			Elapsed:       result.Duration.Round(time.Millisecond).String(),
		},
	}
	report := review.BuildReport(&dummyResult)
	report.Complete = true
	report.Generated = true

	analysis := review.BuildAnalysis(dummyResult)
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

	// Step 6: 写回 PR 评论
	stepWB := newAuditStep("writeback")
	wbSummary, wbErr := e.publishWriteback(ctx, id, req, workspace, domain.Result{Summary: resp.Summary, Findings: findings})
	if wbErr != nil {
		e.appendAuditStep(ctx, id, stepWB.failed("写回失败", "publish findings to git platform", map[string]any{"findings": len(findings)}, wbErr))
	} else {
		e.appendAuditStep(ctx, id, stepWB.done(
			fmt.Sprintf("写回完成：%d 条评论已发布", wbSummary.Published),
			"publish findings as PR review comments",
			map[string]any{"findings": len(findings), "provider": req.Provider, "repo": req.Repo},
			map[string]any{"published": wbSummary.Published, "failed": wbSummary.Failed, "status": wbSummary.Status},
		))
	}
	resp.Writeback = wbSummary
	if wbErr != nil {
		resp.Status = domain.StatusSuccessWithPublishFailed
	}

	if err := e.updateWithRawOCR(ctx, id, req, resp, nil); err != nil {
		return resp, err
	}

	if wbErr != nil {
		return resp, fmt.Errorf("review completed but writeback failed: %w", wbErr)
	}

	e.notifyCompletion(req, resp)
	e.postCompletionStatus(req, resp, workspace)
	e.recordReviewMetrics(req, resp, workspace)
	return resp, nil
}

func (e *Executor) executeWithRunner(ctx context.Context, id string, req domain.Request, workspace domain.Workspace, projectPolicy projectpolicy.ProjectPolicy) (domain.Response, error) {
	// Step 3: OCR 代码审查（核心步骤，耗时最长）
	stepOCR := newAuditStep("ocr")
	ocrInput := map[string]any{
		"dir":        workspace.Dir,
		"base_sha":   workspace.BaseSHA,
		"head_sha":   workspace.HeadSHA,
		"diff_bytes": len(workspace.Diff),
		"scope":      workspace.Scope,
		"rule_path":  workspace.RulePath,
	}
	result, err := e.runner.Run(ctx, workspace)
	if err != nil {
		if e.llmMetrics != nil {
			e.llmMetrics.TotalLatency.Add(int64(time.Since(stepOCR.startTime)))
			e.llmMetrics.Calls.Add(1)
			e.llmMetrics.Failures.Add(1)
		}
		e.appendAuditStep(ctx, id, stepOCR.failed("OCR 代码审查失败", "OCR binary subprocess", ocrInput, err))
		e.failureReporter.Report(ctx, id, req, "ocr", err, result.OCR)
		return domain.Response{}, err
	}
	if e.llmMetrics != nil {
		e.llmMetrics.TotalLatency.Add(int64(time.Since(stepOCR.startTime)))
		e.llmMetrics.Calls.Add(1)
		e.llmMetrics.Successes.Add(1)
		e.llmMetrics.InputTokens.Add(result.OCR.InputTokens)
		e.llmMetrics.OutputTokens.Add(result.OCR.OutputTokens)
	}
	e.appendAuditStep(ctx, id, stepOCR.done(
		fmt.Sprintf("OCR 审查完成：%d 条评论，%d 个文件", result.OCR.Comments, result.OCR.FilesReviewed),
		"OCR binary subprocess (LLM code analysis)",
		ocrInput,
		map[string]any{
			"files_reviewed": result.OCR.FilesReviewed,
			"comments":       result.OCR.Comments,
			"total_tokens":   result.OCR.TotalTokens,
			"input_tokens":   result.OCR.InputTokens,
			"output_tokens":  result.OCR.OutputTokens,
			"elapsed":        result.OCR.Elapsed,
			"findings_count": len(result.Findings),
			"summary":        strutil.Truncate(result.Summary, 500),
		},
	))
	result.Scope = workspace.Scope
	result.Scope.ChangedLines = review.CountDiffLines(workspace.Diff)

	types.ComputeFingerprints(result.Findings)

	// Step 4: OCR 结果二次综合判断（agentkit LLM）
	var judgment *judge.Judgment
	if e.judge != nil {
		stepJudge := newAuditStep("judge")
		judgeInput := judge.FindingsToInput(req.Provider, req.Repo, workspace.Diff, nil)
		judgeInput.ProjectContext = projectPolicy.OCRLanguage
		judgment, judgeErr := e.judge.Analyze(ctx, judgeInput, result)
		if judgeErr != nil {
			logx.Warnf("[executor] judge analysis failed for review %s: %v", id, judgeErr)
		}
		if judgment != nil {
			e.appendAuditStep(ctx, id, stepJudge.done(
				"二次综合判断完成",
				"agentkit LLM (GPT/Claude/mimo) 分析 OCR findings + diff 上下文",
				map[string]any{"findings_count": len(result.Findings), "diff_bytes": len(workspace.Diff), "provider": req.Provider, "repo": req.Repo},
				map[string]any{
					"confidence":       judgment.Confidence,
					"recommended_gate": judgment.RecommendedGate,
					"gate_reason":      judgment.GateReason,
					"findings_count":   len(judgment.PrioritizedFindings),
					"true_positives":   countTruePositives(judgment),
					"false_positives":  len(judgment.PrioritizedFindings) - countTruePositives(judgment),
					"summary":          strutil.Truncate(judgment.SummaryForHuman, 500),
				},
			))
		} else {
			e.appendAuditStep(ctx, id, stepJudge.done("二次判断跳过（LLM 不可用）", "agentkit LLM not configured", nil, nil))
		}
	}

	// Step 5: 门禁评估
	stepGate := newAuditStep("gate")
	report := review.BuildReport(&result)
	analysis := review.BuildAnalysis(result)
	gateEngine := e.policy
	if gateEngine == nil {
		gateEngine = policy.NewEngine(policy.DefaultConfig())
	}
	// 项目 fail_on 是全局门禁默认值的按项目覆盖（合并而非替换，保留全局 IgnorePaths 等）。
	gateEngine = gateEngine.WithFailOnOverride(projectPolicy.FailOn)
	gate := gateEngine.Evaluate(review.PolicyFindings(result.Findings))
	e.appendAuditStep(ctx, id, stepGate.done(
		fmt.Sprintf("门禁评估：%s", gate.Reason),
		fmt.Sprintf("policy engine (fail_on=%s)", projectPolicy.FailOn),
		map[string]any{"findings": len(result.Findings), "policy_fail_on": projectPolicy.FailOn},
		map[string]any{"passed": gate.Passed, "reason": gate.Reason, "counts": report.Counts},
	))

	// 如果 Judge 给出了综合判断，用它的 summary 替换原始 summary
	summary := result.Summary
	if judgment != nil && judgment.SummaryForHuman != "" {
		summary = judgment.SummaryForHuman
	}

	resp := domain.Response{
		ReviewID: id,
		Status:   "success",
		Summary:  summary,
		Report:   report,
		Analysis: analysis,
		Findings: result.Findings,
		Gate:     domain.CompletedGate(gate.Passed, gate.Reason),
		Judgment: judgmentToDomain(judgment),
	}

	wbSummary, wbErr := e.publishWriteback(ctx, id, req, workspace, domain.Result{Summary: result.Summary, Findings: result.Findings})
	resp.Writeback = wbSummary
	if wbErr != nil {
		resp.Status = domain.StatusSuccessWithPublishFailed
	}

	if err := e.updateWithRawOCR(ctx, id, req, resp, result.RawOCR); err != nil {
		if wbErr != nil {
			return resp, fmt.Errorf("review completed but writeback failed: %w", wbErr)
		}
		return domain.Response{}, err
	}

	if wbErr != nil {
		return resp, fmt.Errorf("review completed but writeback failed: %w", wbErr)
	}

	e.notifyCompletion(req, resp)
	e.postCompletionStatus(req, resp, workspace)
	e.recordReviewMetrics(req, resp, workspace)
	return resp, nil
}
