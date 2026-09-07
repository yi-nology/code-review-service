package review

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/yi-nology/code-review-service/internal/domain/projectpolicy"
	"github.com/yi-nology/code-review-service/internal/repository/repoerr"
	"github.com/yi-nology/code-review-service/internal/service/policy"
	"github.com/yi-nology/code-review-service/internal/service/progress"
	"github.com/yi-nology/code-review-service/internal/service/review/domain"
	"github.com/yi-nology/code-review-service/internal/service/types"

	"github.com/oekit/concurrency/async"
	"github.com/oekit/logx"
)

// ScanOrchestrator handles all scan-related operations, extracted from Service
// to reduce complexity and improve separation of concerns.
type ScanOrchestrator struct {
	s *Service
}

// NewScanOrchestrator creates a new ScanOrchestrator
func NewScanOrchestrator(s *Service) *ScanOrchestrator {
	return &ScanOrchestrator{s: s}
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
	if o.s.ScanRunner == nil {
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
	files, err := o.s.previewCore(ctx, domainReq, req.Paths, req.Exclude, false)
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

	id := newReviewID(time.Now().UTC())
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
	if err := o.s.reviewRepo.Create(ctx, reviewRow); err != nil {
		return nil, err
	}

	var reporter progress.ProgressReporter
	if o.s.progressManager != nil {
		tracker := o.s.progressManager.CreateTracker(id, id, 1)
		reporter = tracker
	}

	o.s.inFlight.Add(1)
	async.GoSafe(func() {
		defer o.s.inFlight.Done()
		defer func() {
			if o.s.progressManager != nil {
				o.s.progressManager.RemoveTracker(id)
			}
		}()

		// Get concurrency slot: can be cancelled by ctx to avoid blocking during shutdown.
		if !o.s.acquireConcurrency(ctx) {
			logx.Warnf("scan %s cancelled while waiting for concurrency slot", id)
			return
		}
		defer o.s.releaseConcurrency()

		// PR-03 fix: derive from ctx so scan goroutine receives signal when HTTP request is cancelled.
		// Old context.WithCancel(context.Background()) was completely disconnected from request lifecycle,
		// could only rely on external Cancel() explicit call.
		scanCtx, cancel := context.WithCancel(ctx)
		o.s.scanCancels.Store(id, cancel)
		defer func() {
			cancel()
			o.s.scanCancels.Delete(id)
		}()

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

	if err := o.s.updateReviewFields(scanCtx, reviewID, func(review *domain.Record) {
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

	p, ok := o.s.Providers.Get(providerName)
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
	if o.s.projectRepo != nil {
		if policyRecord, err := o.s.projectRepo.GetOrCreateDefault(ctx, providerName, repo); err == nil {
			projectPolicy = policyRecord
		} else if !errors.Is(err, repoerr.ErrNotFound) {
			logx.Warn("failed to get/create project policy, using default", "error", err)
		}
	}
	workspace.OCR = ProjectPolicyToOCROverrides(projectPolicy)
	if strings.TrimSpace(req.Background) != "" {
		workspace.OCR.Background = req.Background
	}

	if o.s.globalRulesRepo != nil {
		rulePath, cleanup, ruleErr := ResolveRuleFileForAgents(ctx, projectPolicy, o.s.rulePackageRepo, o.s.globalRulesRepo, AgentRulePackageRefs(ctx, o.s.agentRepo)...)
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
			Binary: ResolveOCRBinary(ctx, o.s.ocrSettingsRepo),
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
		if err := o.s.updateReviewFields(ctx, reviewID, func(review *domain.Record) {
			rawOCRInputJSON, _ := json.Marshal(ocrInput)
			review.RawOCRInput = rawOCRInputJSON
		}); err != nil {
			logx.Warn("failed to persist scan ocr input", "review_id", reviewID, "error", err)
		}
	}

	scanResult, err := o.s.ScanRunner.Scan(ctx, workspace, req)
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
	err := o.s.updateReviewFields(ctx, reviewID, func(review *domain.Record) {
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

	gateEngine := o.s.policy
	if gateEngine == nil {
		gateEngine = policy.NewEngine(policy.Config{})
	}
	if projectPolicy.FailOn != "" {
		gateEngine = policy.NewEngine(policy.Config{FailOn: projectPolicy.FailOn})
	}
	gate := gateEngine.Evaluate(scanPolicyFindings(result.Findings))

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

	return o.s.updateReviewFields(ctx, reviewID, func(review *domain.Record) {
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

// scanPolicyFindings converts domain findings to policy findings
func scanPolicyFindings(findings []domain.Finding) []policy.Finding {
	out := make([]policy.Finding, 0, len(findings))
	for _, finding := range findings {
		out = append(out, policy.Finding{Severity: finding.Severity, Path: finding.Path})
	}
	return out
}
