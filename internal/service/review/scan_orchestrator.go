package review

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	types "github.com/yi-nology/code-review-service/internal/domain/finding"
	"github.com/yi-nology/code-review-service/internal/domain/projectpolicy"
	"github.com/yi-nology/code-review-service/internal/domain/review"
	"github.com/yi-nology/code-review-service/internal/repository"
	"github.com/yi-nology/code-review-service/internal/repository/repoerr"
	"github.com/yi-nology/code-review-service/internal/service/policy"
	"github.com/yi-nology/code-review-service/internal/service/progress"

	"github.com/oekit/logx"
)

// updateReviewFields 事务内按 reviewID 修改评审记录（与 Service.updateReviewFields 同一仓储路径）。
func (o *ScanOrchestrator) updateReviewFields(ctx context.Context, reviewID string, fn func(review *domain.Record)) error {
	return o.store.UpdateInTransaction(ctx, reviewID, func(review *domain.Record) error {
		fn(review)
		return nil
	})
}

// ScanOrchestrator 负责所有 scan 相关操作（preview/异步执行/结果落库）。
// 通过共享 runtime 使用并发/取消设施，其余依赖在构造期显式注入——
// 不再持有整只 *Service。
type ScanOrchestrator struct {
	s          *Service
	store      ReviewStateStore
	providers  domain.Providers
	scanRunner domain.ScanRunner
	policy     *policy.Engine
	progress   *progress.ProgressManager
	runtime    *scanRuntime
	// 规则/OCR 配置解析所需的仓储（构造期从 Service 快照，只读使用）
	projectRepo     repository.ProjectPolicyRepository
	globalRulesRepo repository.GlobalRulesRepository
	rulePackageRepo repository.OCRRulePackageRepository
	agentRepo       repository.ReviewerAgentRepository
	ocrSettingsRepo repository.OCRSettingsRepository
	// previewFn 由 Service 注入（previewCore 与 Service 内部共享状态）
	previewFn func(ctx context.Context, req domain.Request, paths, exclude []string, includeAgentRules bool) ([]string, error)
}

// NewScanOrchestrator creates a new ScanOrchestrator
func NewScanOrchestrator(s *Service) *ScanOrchestrator {
	return &ScanOrchestrator{
		s:               s,
		store:           s.reviewRepo,
		providers:       s.Providers,
		scanRunner:      s.ScanRunner,
		policy:          s.policy,
		progress:        s.progressManager,
		runtime:         s.runtime,
		projectRepo:     s.projectRepo,
		globalRulesRepo: s.globalRulesRepo,
		rulePackageRepo: s.rulePackageRepo,
		agentRepo:       s.agentRepo,
		ocrSettingsRepo: s.ocrSettingsRepo,
		previewFn:       s.previewCore,
	}
}

// Scan executes a repository scan with preview support
func (o *ScanOrchestrator) Scan(ctx context.Context, req domain.ScanRequest) (*domain.ScanResult, error) {
	providerName := strings.TrimSpace(req.Provider)
	repo := strings.TrimSpace(req.Repo)
	head := strings.TrimSpace(req.HeadSHA)
	if providerName == "" {
		return nil, fmt.Errorf("provider is required")
	}
	if repo == "" {
		return nil, fmt.Errorf("repo is required")
	}
	if head == "" {
		return nil, fmt.Errorf("head_sha is required")
	}
	if o.scanRunner == nil {
		return nil, fmt.Errorf("scan feature not configured")
	}

	if req.Preview {
		return o.scanPreview(ctx, req)
	}

	return o.scanAsync(ctx, req)
}

// scanPreview handles lightweight preview path: clone + ocr scan --preview (returns file list in seconds).
// Does not call LLM. Previously using scanExecuteWithPolicy would run full OCR scan (3-7 minutes),
// causing frontend 120s timeout.
//
// Scan mode has no PR diff scope concept, so BaseSHA and HeadSHA being the same is reasonable.
// If we need to support 'scan preview for specific base..head range' in the future, we need to add
// BaseSHA field to domain.ScanRequest and modify here.
func (o *ScanOrchestrator) scanPreview(ctx context.Context, req domain.ScanRequest) (*domain.ScanResult, error) {
	domainReq := domain.Request{
		Provider: strings.TrimSpace(req.Provider),
		Repo:     strings.TrimSpace(req.Repo),
		BaseSHA:  strings.TrimSpace(req.HeadSHA),
		HeadSHA:  strings.TrimSpace(req.HeadSHA),
	}
	// Scan doesn't call LLM, skip agent rule package collection to avoid one ListEnabled query
	// and prevent polluting ocr scan --preview input.
	files, err := o.previewFn(ctx, domainReq, req.Paths, req.Exclude, false)
	if err != nil {
		return nil, err
	}
	return &domain.ScanResult{PreviewFiles: files}, nil
}

// scanAsync handles async scan execution with background goroutine management
func (o *ScanOrchestrator) scanAsync(ctx context.Context, req domain.ScanRequest) (*domain.ScanResult, error) {
	providerName := strings.TrimSpace(req.Provider)
	repo := strings.TrimSpace(req.Repo)
	head := strings.TrimSpace(req.HeadSHA)

	id := NewReviewID(time.Now().UTC())
	scanReq := domain.Request{
		Provider:     providerName,
		Repo:         repo,
		ProjectID:    strings.TrimSpace(req.ProjectID),
		BaseSHA:      head,
		HeadSHA:      head,
		CallbackMode: "scan",
	}
	fingerprint, err := requestFingerprint(scanReq)
	if err != nil {
		return nil, err
	}

	queuedResp := domain.Response{
		ReviewID: id,
		Status:   StatusQueued,
		Summary:  "repository scan queued",
		Gate:     domain.PendingGate(),
		History: []domain.Event{{
			Stage:     stageQueued,
			Status:    statusCompleted,
			Message:   "repository scan queued",
			CreatedAt: time.Now().UTC(),
		}},
	}

	reviewRow := &domain.Record{
		ID:                 id,
		RequestFingerprint: fingerprint,
		Provider:           providerName,
		Repo:               repo,
		Status:             StatusQueued,
		BaseSHA:            head,
		HeadSHA:            head,
		Summary:            queuedResp.Summary,
		Request:            scanReq,
		Response:           queuedResp,
	}
	if err := o.store.Create(ctx, reviewRow); err != nil {
		return nil, err
	}

	var reporter progress.ProgressReporter
	if o.progress != nil {
		tracker := o.progress.CreateTracker(id, id, 1)
		reporter = tracker
	}

	// 后台执行：占用并发槽、登记 in-flight、注册可取消扫描，
	// 生命周期设施统一走共享 scanRuntime（与 worker 同一套）。
	o.runtime.runDetached(ctx, id, func(scanCtx context.Context) {
		if o.progress != nil {
			defer o.progress.RemoveTracker(id)
		}
		o.runScanBackground(scanCtx, id, req, reporter)
	})

	return &domain.ScanResult{ReviewID: id}, nil
}

// runScanBackground executes the scan in background goroutine
func (o *ScanOrchestrator) runScanBackground(scanCtx context.Context, reviewID string, req domain.ScanRequest, reporter progress.ProgressReporter) {
	if reporter != nil {
		scanCtx = progress.WithProgressReporter(scanCtx, reporter)
		reporter.UpdatePhase(progress.PhaseContext, "active", 3, "准备仓库工作区...")
	}

	if err := o.updateReviewFields(scanCtx, reviewID, func(review *domain.Record) {
		resp := review.Response
		if resp.ReviewID == "" {
			resp = domain.Response{ReviewID: reviewID}
		}
		resp.Status = statusRunning
		resp.Summary = "repository scan running"
		resp.History = append(resp.History, domain.Event{
			Stage:     "repository",
			Status:    statusRunning,
			Message:   "preparing workspace and running full scan",
			CreatedAt: time.Now().UTC(),
		})
		review.Status = statusRunning
		review.Response = resp
		review.UpdatedAt = time.Now().UTC()
	}); err != nil {
		logx.Errorf("failed to update scan status to running: review_id=%s: %v", reviewID, err)
	}

	// 执行期心跳：扫描同样可能长时间无落库事件，防止被 stale 重排误判为孤儿
	// （孤儿 scan 会被 worker 认领后直接置 failed，而真实扫描仍在跑）。
	stopHeartbeat := o.s.startHeartbeat(reviewID, o.s.heartbeatInterval)
	defer stopHeartbeat()

	scanResult, projectPolicy, err := o.scanExecuteWithPolicy(scanCtx, reviewID, req)
	if err != nil {
		logx.Errorf("async scan failed: review_id=%s: %v", reviewID, err)
		o.recordScanFailure(scanCtx, reviewID, err)
		if tracker, ok := reporter.(*progress.ProgressTracker); ok {
			tracker.SetError(err.Error())
		}
		return
	}

	if err := o.persistScanResultUpdate(scanCtx, reviewID, req, projectPolicy, scanResult); err != nil {
		logx.Errorf("failed to persist scan result: review_id=%s: %v", reviewID, err)
		o.recordScanFailure(scanCtx, reviewID, err)
		if tracker, ok := reporter.(*progress.ProgressTracker); ok {
			tracker.SetError(err.Error())
		}
		return
	}

	if tracker, ok := reporter.(*progress.ProgressTracker); ok {
		tracker.UpdatePhase(progress.PhaseSynthesis, "active", 90, "保存扫描结果...")
		tracker.Complete()
	}
}

// scanExecuteWithPolicy executes scan with project policy resolution
func (o *ScanOrchestrator) scanExecuteWithPolicy(ctx context.Context, reviewID string, req domain.ScanRequest) (*domain.ScanResult, projectpolicy.ProjectPolicy, error) {
	providerName := strings.TrimSpace(req.Provider)
	repo := strings.TrimSpace(req.Repo)
	head := strings.TrimSpace(req.HeadSHA)

	reporter := progress.ProgressReporterFromContext(ctx)

	p, ok := o.providers.Get(providerName)
	if !ok {
		logx.Errorf("Scan: provider not found: requested=%q", providerName)
		return nil, projectpolicy.ProjectPolicy{}, fmt.Errorf("%w: %s", domain.ErrUnsupportedProvider, providerName)
	}

	if reporter != nil {
		reporter.UpdatePhase(progress.PhaseContext, "active", 5, "克隆/检出仓库...")
	}
	workspace, err := p.Prepare(ctx, domain.Request{
		Provider: providerName,
		Repo:     repo,
		BaseSHA:  head,
		HeadSHA:  head,
	})
	if err != nil {
		return nil, projectpolicy.ProjectPolicy{}, err
	}
	if workspace.Dir != "" {
		defer func() { _ = os.RemoveAll(workspace.Dir) }()
	}

	projectPolicy := projectpolicy.NewDefault(providerName, repo)
	if o.projectRepo != nil {
		if policyRecord, err := o.projectRepo.GetOrCreateDefault(ctx, providerName, repo); err == nil {
			projectPolicy = policyRecord
		} else if !errors.Is(err, repoerr.ErrNotFound) {
			logx.Warn("failed to get/create project policy, using default", "error", err)
		}
	}
	workspace.OCR = ProjectPolicyToOCROverrides(projectPolicy)
	if strings.TrimSpace(req.Background) != "" {
		workspace.OCR.Background = req.Background
	}

	if o.globalRulesRepo != nil {
		rulePath, cleanup, ruleErr := ResolveRuleFileForAgents(ctx, projectPolicy, o.rulePackageRepo, o.globalRulesRepo, AgentRulePackageRefs(ctx, o.agentRepo)...)
		if ruleErr != nil {
			logx.Warn("failed to resolve rule file", "error", ruleErr)
		} else if rulePath != "" {
			workspace.RulePath = rulePath
			if cleanup != nil {
				defer cleanup()
			}
		}
	}

	if reporter != nil {
		reporter.UpdatePhase(progress.PhaseAnalysis, "active", 10, "启动代码扫描引擎...")
	}

	if reviewID != "" && workspace.Dir != "" {
		ocrInput := domain.OCRInput{
			Binary: ResolveOCRBinary(ctx, o.ocrSettingsRepo),
			Workspace: domain.WorkspaceInfo{
				Provider: providerName,
				Repo:     repo,
				BaseSHA:  workspace.BaseSHA,
				HeadSHA:  workspace.HeadSHA,
				Dir:      workspace.Dir,
			},
			RulePath:  workspace.RulePath,
			StartedAt: time.Now().UTC(),
		}
		if err := WriteOCRInput(ctx, o.store, reviewID, ocrInput); err != nil {
			logx.Warn("failed to persist scan ocr input", "review_id", reviewID, "error", err)
		}
	}

	scanResult, err := o.scanRunner.Scan(ctx, workspace, req)
	if err != nil {
		return nil, projectpolicy.ProjectPolicy{}, err
	}
	if req.Preview || scanResult == nil || scanResult.Result == nil {
		return scanResult, projectPolicy, nil
	}

	if reporter != nil {
		reporter.UpdatePhase(progress.PhaseSynthesis, "active", 80, "生成扫描报告...")
	}
	report := BuildReport(scanResult.Result)
	analysis := BuildAnalysis(*scanResult.Result)
	scanResult.Report = &report
	scanResult.Analysis = &analysis

	return scanResult, projectPolicy, nil
}

// recordScanFailure records scan failure in database
func (o *ScanOrchestrator) recordScanFailure(ctx context.Context, reviewID string, scanErr error) {
	err := o.updateReviewFields(ctx, reviewID, func(review *domain.Record) {
		resp := review.Response
		if resp.ReviewID == "" {
			resp = domain.Response{ReviewID: reviewID}
		}
		resp.Status = statusFailed
		resp.Summary = "repository scan failed"
		resp.Gate = domain.ErrorGate("scan execution failed")
		resp.Error = scanErr.Error()
		resp.History = append(resp.History, domain.Event{
			Stage:     "ocr_scan",
			Status:    statusFailed,
			Message:   scanErr.Error(),
			CreatedAt: time.Now().UTC(),
		})
		review.Status = statusFailed
		review.Response = resp
		review.UpdatedAt = time.Now().UTC()
	})
	if err != nil {
		logx.Errorf("failed to record scan failure: review_id=%s: %v", reviewID, err)
	}
}

// persistScanResultUpdate persists scan result to database
func (o *ScanOrchestrator) persistScanResultUpdate(ctx context.Context, reviewID string, scanReq domain.ScanRequest, projectPolicy projectpolicy.ProjectPolicy, scanResult *domain.ScanResult) error {
	result := scanResult.Result
	if result == nil {
		return fmt.Errorf("scan result is empty")
	}
	// Compute stable fingerprints across reviews before gate/report/writeback consumes findings.
	types.ComputeFingerprints(result.Findings)
	report := BuildReport(result)
	if scanResult.Report != nil {
		report = *scanResult.Report
	}
	analysis := BuildAnalysis(*result)
	if scanResult.Analysis != nil {
		analysis = *scanResult.Analysis
	}

	gateEngine := o.policy
	if gateEngine == nil {
		gateEngine = policy.NewEngine(policy.DefaultConfig())
	}
	// 项目 fail_on 覆盖全局默认值（合并语义，保留 IgnorePaths 等）。
	gateEngine = gateEngine.WithFailOnOverride(projectPolicy.FailOn)
	gate := gateEngine.Evaluate(PolicyFindings(result.Findings))

	resp := domain.Response{
		ReviewID: reviewID,
		Status:   statusSuccess,
		Summary:  result.Summary,
		Gate:     domain.CompletedGate(gate.Passed, gate.Reason),
		Report:   report,
		Analysis: analysis,
		Findings: result.Findings,
		History: []domain.Event{
			{
				Stage:     "repository",
				Status:    statusCompleted,
				Message:   "repository prepared for full scan",
				CreatedAt: time.Now().UTC(),
			},
			{
				Stage:   "ocr_scan",
				Status:  statusCompleted,
				Message: "full repository scan completed",
				Details: map[string]any{
					"files_reviewed": result.OCR.FilesReviewed,
					"comments":       result.OCR.Comments,
					"total_tokens":   result.OCR.TotalTokens,
					"elapsed":        result.OCR.Elapsed,
				},
				CreatedAt: time.Now().UTC(),
			},
		},
	}

	return o.updateReviewFields(ctx, reviewID, func(review *domain.Record) {
		review.Status = resp.Status
		review.Score = report.Score.Value
		review.Grade = report.Score.Grade
		review.Summary = resp.Summary
		review.Response = resp
		if len(result.RawOCR) > 0 {
			review.RawOCR = result.RawOCR
		}
		review.UpdatedAt = time.Now().UTC()
	})
}
