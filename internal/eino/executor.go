package eino

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/cloudwego/eino/callbacks"
	types "github.com/yi-nology/code-review-service/internal/domain/finding"
	"github.com/yi-nology/code-review-service/internal/ocr"
	"github.com/yi-nology/code-review-service/internal/service/agent"
	"github.com/yi-nology/code-review-service/internal/service/cost"
	"github.com/yi-nology/code-review-service/internal/service/judge"
	"github.com/yi-nology/code-review-service/internal/service/memory"
	"github.com/yi-nology/code-review-service/internal/service/policy"
	"github.com/yi-nology/code-review-service/internal/service/progress"
	"github.com/yi-nology/code-review-service/internal/service/review"
	"github.com/yi-nology/code-review-service/internal/service/review/domain"
	"github.com/yi-nology/code-review-service/internal/service/statuscheck"
	"github.com/yi-nology/code-review-service/internal/service/workflow"
	"github.com/yi-nology/code-review-service/internal/strutil"

	agentkitllm "github.com/oagentkit/llm"
	"github.com/oagentkit/llm/obsx"
	"github.com/oekit/logx"
)

// Package eino：评审执行器核心——Execute 主流程与两条引擎路径（workflow/单 runner）。
// Executor implements the review.Executor interface using eino workflows.
// This replaces the old executor.Service with a cleaner, eino-based implementation.
type Executor struct {
	// pipeline 是评审的**前置编排**端口（克隆工作区 → 项目策略 → 文件过滤 →
	// 规则合并 → 记忆召回 → OCR 执行参数）。实现在 service/review。
	//
	// 此前这段编排住在本包（引擎适配层），于是本包直接持 6 个仓储、绕过 service
	// 层直读业务表，又反向调用 service/review 的 13 个导出函数——两套编排互相
	// 调用，是耦合的主要来源。现在引擎侧只认这个端口。
	pipeline review.PipelineRunner
	// store 是评审状态的窄存储接口（评审记录读写）。用窄接口而非完整的
	// repository.ReviewRepository：本包不再需要仓储包，依赖方向不再倒置。
	store           review.ReviewStateStore
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
	memory          *memory.Service         // 跨会话长期记忆（可 nil = 不启用）

	// running tracks currently executing reviews for cancellation support
	runningMu sync.Mutex
	running   map[string]context.CancelFunc
}

// ExecutorConfig contains configuration for the eino executor
type ExecutorConfig struct {
	// Pipeline 前置编排端口（由 service/review 实现，装配层注入）。
	Pipeline review.PipelineRunner
	// Store 评审状态存储（窄接口）。
	Store         review.ReviewStateStore
	Runner        domain.Runner
	Writeback     domain.WritebackPublisher
	Notifier      domain.Notifier
	StatusPoster  *statuscheck.Poster
	ModelNameFunc func() string
	BudgetChecker *cost.BudgetChecker
	CostCalc      *cost.Calculator
	Policy        *policy.Engine
	ProgressMgr   *progress.ProgressManager
	Workflow      *workflow.ReviewWorkflow
	AgentManager  *agent.AgentManager
	LLMMetrics    *ocr.Metrics
	Judge         *judge.Judge // OCR 结果二次综合判断（可 nil）
	// FailureReporter 统一失败上报；nil 时用 Notifier/StatusPoster 兜底构造
	//（保持旧行为），两者也缺时退化为仅落库。
	FailureReporter *review.FailureReporter
	// Memory 跨会话长期记忆（可 nil = 不启用）。
	Memory *memory.Service
}

// NewExecutor creates a new eino-based executor
func NewExecutor(cfg ExecutorConfig) *Executor {
	failureReporter := cfg.FailureReporter
	if failureReporter == nil {
		failureReporter = review.NewFailureReporter(cfg.Store, cfg.Notifier, cfg.StatusPoster)
	}
	return &Executor{
		pipeline:        cfg.Pipeline,
		store:           cfg.Store,
		judge:           cfg.Judge,
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
		memory:          cfg.Memory,
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

	// 启用 eino LLM 调用追踪（结构化日志：duration/tokens/慢调用告警）+
	// token 用量记账（UsageHandler 真实 usage 单源，注入 llmMetrics——覆盖
	// ReAct agent 直用 BaseChatModel 的场景）。agentkit v0.10.12 起
	// 记账单源化：原 obsx.Options.OnUsage 已移除，改经 llm.NewUsageHandler。
	ctx = callbacks.InitCallbacks(ctx, nil,
		obsx.NewTracingHandler(logx.Default(), obsx.Options{SlowThreshold: 30 * time.Second}),
		agentkitllm.NewUsageHandler(func(rec agentkitllm.UsageRecord) {
			if e.llmMetrics != nil {
				e.llmMetrics.InputTokens.Add(int64(rec.PromptTokens))
				e.llmMetrics.OutputTokens.Add(int64(rec.CompletionTokens))
			}
		}),
	)

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
	e.notifyStart(ctx, id, req)

	// Budget gate
	if e.budgetChecker != nil {
		if err := e.budgetChecker.CheckBlock(ctx); err != nil {
			e.recordFailure(ctx, id, req, "budget", err)
			return domain.Response{}, err
		}
	}

	// 前置编排（Step 0 ~ 2.8）下沉到 service/review 的 Pipeline：
	// 克隆工作区 → 项目策略 → 文件过滤 → 规则合并 → 记忆召回 → OCR 执行参数。
	//
	// 引擎侧只负责：起引擎上下文、管编排生命周期、调 workflow / 单 runner。
	useWorkflow := e.shouldUseWorkflow()
	out, err := e.pipeline.Run(ctx, review.PipelineInput{
		ReviewID:         id,
		Request:          req,
		Provider:         provider,
		ProgressReporter: progressReporter,
		UseWorkflow:      useWorkflow,
		ExecutionMode:    e.executionMode(useWorkflow),
	})
	if err != nil {
		stage := review.StageOf(err)
		if stage == "" {
			stage = "pipeline"
		}
		e.recordFailure(ctx, id, req, stage, err)
		return domain.Response{}, err
	}
	// 工作区目录与合并后的规则文件在引擎执行期间必须存活，
	// 因此清理责任留在调用方而不是管线内部。
	defer out.Cleanup()

	if useWorkflow {
		return e.executeWithWorkflow(ctx, id, req, out)
	}
	return e.executeWithRunner(ctx, id, req, out)
}

// executionMode 返回执行模式的可读描述（写进审计步骤）。
func (e *Executor) executionMode(useWorkflow bool) string {
	if useWorkflow {
		return "multi-agent workflow"
	}
	return "single-agent runner"
}

func (e *Executor) executeWithWorkflow(
	ctx context.Context,
	id string,
	req domain.Request,
	out review.PipelineOutput,
) (domain.Response, error) {
	workspace := out.Workspace
	memoryContext := out.MemoryContext
	e.appendHistory(ctx, id, "workflow", "running", "multi-agent review workflow started", nil)

	wfReq := workflow.ReviewRequest{
		ReviewID:      id,
		Provider:      req.Provider,
		Repo:          req.Repo,
		MemoryContext: memoryContext,
		BaseSHA:       workspace.BaseSHA,
		HeadSHA:       workspace.HeadSHA,
		Diff:          workspace.Diff,
		Dir:           workspace.Dir,
		Round:         1,
		Requirements:  req.Requirements,
		Language:      workspace.OCR.Language,
		RulePath:      workspace.RulePath,
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

	// 把本次评审沉淀进跨会话长期记忆（异步，见 captureReviewMemoryAsync）。
	e.captureReviewMemoryAsync(ctx, id, req, workspace, domain.Result{
		Summary:  result.FinalReport.Summary,
		Findings: findings,
	}, result.FinalReport.GatePassed)

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
	stepWB := review.NewAuditStep("writeback")
	wbSummary, wbErr := e.publishWriteback(ctx, id, req, workspace, domain.Result{Summary: resp.Summary, Findings: findings})
	if wbErr != nil {
		e.appendAuditStep(ctx, id, stepWB.Failed("写回失败", "publish findings to git platform", map[string]any{"findings": len(findings)}, wbErr))
	} else {
		e.appendAuditStep(ctx, id, stepWB.Done(
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

func (e *Executor) executeWithRunner(ctx context.Context, id string, req domain.Request, out review.PipelineOutput) (domain.Response, error) {
	workspace := out.Workspace
	projectPolicy := out.ProjectPolicy
	// Step 3: OCR 代码审查（核心步骤，耗时最长）
	stepOCR := review.NewAuditStep("ocr")
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
			e.llmMetrics.TotalLatency.Add(int64(stepOCR.Elapsed()))
			e.llmMetrics.Calls.Add(1)
			e.llmMetrics.Failures.Add(1)
		}
		e.appendAuditStep(ctx, id, stepOCR.Failed("OCR 代码审查失败", "OCR binary subprocess", ocrInput, err))
		e.failureReporter.Report(ctx, id, req, "ocr", err, result.OCR)
		return domain.Response{}, err
	}
	if e.llmMetrics != nil {
		e.llmMetrics.TotalLatency.Add(int64(stepOCR.Elapsed()))
		e.llmMetrics.Calls.Add(1)
		e.llmMetrics.Successes.Add(1)
		e.llmMetrics.InputTokens.Add(result.OCR.InputTokens)
		e.llmMetrics.OutputTokens.Add(result.OCR.OutputTokens)
	}
	e.appendAuditStep(ctx, id, stepOCR.Done(
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
		stepJudge := review.NewAuditStep("judge")
		judgeInput := judge.FindingsToInput(req.Provider, req.Repo, workspace.Diff, nil)
		judgeInput.ProjectContext = projectPolicy.OCRLanguage
		judgment, judgeErr := e.judge.Analyze(ctx, judgeInput, result)
		if judgeErr != nil {
			logx.Warnf("[executor] judge analysis failed for review %s: %v", id, judgeErr)
		}
		if judgment != nil {
			e.appendAuditStep(ctx, id, stepJudge.Done(
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
			e.appendAuditStep(ctx, id, stepJudge.Done("二次判断跳过（LLM 不可用）", "agentkit LLM not configured", nil, nil))
		}
	}

	// Step 5: 门禁评估
	stepGate := review.NewAuditStep("gate")
	report := review.BuildReport(&result)
	analysis := review.BuildAnalysis(result)
	gateEngine := e.policy
	if gateEngine == nil {
		gateEngine = policy.NewEngine(policy.DefaultConfig())
	}
	// 项目 fail_on 是全局门禁默认值的按项目覆盖（合并而非替换，保留全局 IgnorePaths 等）。
	gateEngine = gateEngine.WithFailOnOverride(projectPolicy.FailOn)
	gate := gateEngine.Evaluate(review.PolicyFindings(result.Findings))
	e.appendAuditStep(ctx, id, stepGate.Done(
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

	// 把本次评审沉淀进跨会话长期记忆（异步，见 captureReviewMemoryAsync）。
	e.captureReviewMemoryAsync(ctx, id, req, workspace, domain.Result{
		Summary:  summary,
		Findings: result.Findings,
	}, gate.Passed)

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
