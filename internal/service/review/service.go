package review

import (
	"context"
	crypto_rand "crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/yi-nology/code-review-service/internal/repository"
	"github.com/yi-nology/code-review-service/internal/repository/postgres/models"
	"github.com/yi-nology/code-review-service/internal/service/policy"
	"github.com/yi-nology/code-review-service/internal/service/progress"
	gitprovider "github.com/yi-nology/code-review-service/internal/service/provider"
	"github.com/yi-nology/code-review-service/internal/service/review/domain"

	"github.com/oekit/concurrency/async"
	mw "github.com/oekit/middleware"

	"github.com/oekit/gormx" //lint:ignore SA1019 PageResponse 未废弃，上游包级别注释误标
	"github.com/oekit/logx"
	"github.com/oekit/pkg/ptrx"
	"gorm.io/datatypes"
)

var gitObjectIDPattern = regexp.MustCompile(`^[0-9a-fA-F]{7,64}$`)

var errIdempotencyKeyTooLong = mw.NewBadRequestError("idempotency key too long")

var (
	ErrNotFound     = repository.ErrNotFound
	ErrInvalidState = repository.ErrInvalidState
)

const (
	idempotencyKeyMaxLength = 200

	StatusQueued    = "queued"
	statusRunning   = "running"
	statusSuccess   = "success"
	statusFailed    = "failed"
	statusCompleted = "completed"

	stageQueued    = "queued"
	stageCancelled = "cancelled"
)

type Executor interface {
	Execute(ctx context.Context, id string, req domain.Request, provider domain.Provider, progressReporter progress.ProgressReporter) (domain.Response, error)
}

type SessionRepo interface {
	repository.SessionCanceller
}

type ServiceConfig struct {
	ReviewRepo      repository.ReviewRepository
	OCRSettingsRepo repository.OCRSettingsRepository
	ProjectRepo     repository.ProjectPolicyRepository
	GlobalRulesRepo repository.GlobalRulesRepository
	RulePackageRepo repository.OCRRulePackageRepository
	SessionRepo     SessionRepo
	GitProviderSvc  *gitprovider.GitProviderService
	Policy          *policy.Engine
	Providers       domain.Providers
	Runner          domain.Runner
	PreviewRunner   domain.PreviewRunner
	ScanRunner      domain.ScanRunner
	ProgressManager *progress.ProgressManager
	Executor        Executor
}

type Service struct {
	reviewRepo      repository.ReviewRepository
	ocrSettingsRepo repository.OCRSettingsRepository
	projectRepo     repository.ProjectPolicyRepository
	globalRulesRepo repository.GlobalRulesRepository
	rulePackageRepo repository.OCRRulePackageRepository
	sessionRepo     SessionRepo
	gitProviderSvc  *gitprovider.GitProviderService
	policy          *policy.Engine
	Providers       domain.Providers
	runner          domain.Runner
	PreviewRunner   domain.PreviewRunner
	ScanRunner      domain.ScanRunner
	progressManager *progress.ProgressManager
	Executor        Executor
	inFlight        sync.WaitGroup
	concurrencySem  chan struct{}
	scanCancels     sync.Map
}

func NewService(cfg ServiceConfig) *Service {
	return &Service{
		reviewRepo:      cfg.ReviewRepo,
		ocrSettingsRepo: cfg.OCRSettingsRepo,
		projectRepo:     cfg.ProjectRepo,
		globalRulesRepo: cfg.GlobalRulesRepo,
		rulePackageRepo: cfg.RulePackageRepo,
		sessionRepo:     cfg.SessionRepo,
		gitProviderSvc:  cfg.GitProviderSvc,
		policy:          cfg.Policy,
		Providers:       cfg.Providers,
		runner:          cfg.Runner,
		PreviewRunner:   cfg.PreviewRunner,
		ScanRunner:      cfg.ScanRunner,
		progressManager: cfg.ProgressManager,
		Executor:        cfg.Executor,
	}
}

func (s *Service) Create(ctx context.Context, req domain.Request, idempotencyKey string) (domain.Response, error) {
	return s.createReview(ctx, req, idempotencyKey, false)
}

func (s *Service) CreateFromWebhook(ctx context.Context, req domain.Request, idempotencyKey string) (domain.Response, error) {
	return s.createReview(ctx, req, idempotencyKey, true)
}

func (s *Service) createReview(ctx context.Context, req domain.Request, idempotencyKey string, isWebhook bool) (domain.Response, error) {
	if err := validate(req); err != nil {
		return domain.Response{}, err
	}
	idempotencyKey = strings.TrimSpace(idempotencyKey)
	if len(idempotencyKey) > idempotencyKeyMaxLength {
		return domain.Response{}, errIdempotencyKeyTooLong
	}
	fingerprint, err := requestFingerprint(req)
	if err != nil {
		return domain.Response{}, err
	}

	provider, ok := s.Providers.Get(req.Provider)
	if !ok {
		logx.Errorf("Create: provider not found: requested=%q", req.Provider)
		return domain.Response{}, fmt.Errorf("%w: %q", domain.ErrUnsupportedProvider, req.Provider)
	}

	if existing := s.findActiveReview(ctx, req); existing != nil {
		return *existing, nil
	}

	id := newReviewID(time.Now().UTC())
	queued := domain.Response{
		ReviewID: id,
		Status:   StatusQueued,
		Summary:  queuedSummary(isWebhook),
		Gate:     domain.PendingGate(),
		History: []domain.Event{{
			Stage:     stageQueued,
			Status:    statusCompleted,
			Message:   queuedMessage(isWebhook),
			CreatedAt: time.Now().UTC(),
		}},
	}

	review := &models.Review{
		ReviewID:           id,
		RequestFingerprint: fingerprint,
		Provider:           req.Provider,
		Repo:               req.Repo,
		Status:             StatusQueued,
		Author:             req.Author,
		AuthorName:         req.AuthorName,
		AuthorAvatar:       req.AuthorAvatar,
		Sender:             req.Sender,
		PullRequestID:      req.PullRequestID,
		MergeRequestIID:    req.MergeRequestIID,
		BaseSHA:            req.BaseSHA,
		HeadSHA:            req.HeadSHA,
		Summary:            queued.Summary,
		Request:            mustMarshal(req),
		Response:           mustMarshal(queued),
	}
	if idempotencyKey != "" {
		review.IdempotencyKey = &idempotencyKey
	}

	if err := s.reviewRepo.Create(ctx, review); err != nil {
		if idempotencyKey != "" && gormx.IsUniqueViolation(err) {
			existing, getErr := s.reviewRepo.GetByIdempotencyKey(ctx, idempotencyKey)
			if getErr != nil && !errors.Is(getErr, repository.ErrNotFound) {
				return domain.Response{}, fmt.Errorf("idempotency recovery: %w", getErr)
			}
			if existing != nil {
				if existing.RequestFingerprint != fingerprint {
					return domain.Response{}, domain.ErrIdempotencyConflict
				}
				var resp domain.Response
				if err := json.Unmarshal(existing.Response, &resp); err != nil {
					return domain.Response{}, fmt.Errorf("decode existing review response: %w", err)
				}
				return resp, nil
			}
		}
		// 部分唯一索引 idx_reviews_active_change 冲突：并发创建同一 PR/MR 的 active review，
		// 返回已存在的 active review 而非报错（#24 TOCTOU 修复）。
		if gormx.IsUniqueViolation(err) {
			if existing := s.findActiveReview(ctx, req); existing != nil {
				logx.CtxWarnf(ctx, "concurrent review create for same PR detected, returning existing active review: %s", existing.ReviewID)
				return *existing, nil
			}
		}
		return domain.Response{}, err
	}

	if !req.Wait {
		return queued, nil
	}

	return s.Executor.Execute(ctx, id, req, provider, nil)
}

func (s *Service) findActiveReview(ctx context.Context, req domain.Request) *domain.Response {
	hasPR := req.PullRequestID != "" || req.MergeRequestIID != ""
	if !hasPR {
		return nil
	}
	existing, err := s.reviewRepo.GetActiveByProviderRepo(ctx, req.Provider, req.Repo)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil
		}
		logx.CtxErrorf(ctx, "failed to query active review by provider=%s repo=%s: %v", req.Provider, req.Repo, err)
		return nil
	}
	var existingReq domain.Request
	if err := json.Unmarshal(existing.Request, &existingReq); err != nil {
		logx.CtxWarnf(ctx, "failed to decode active review request: %v", err)
		return nil
	}
	if existingReq.PullRequestID != req.PullRequestID || existingReq.MergeRequestIID != req.MergeRequestIID {
		return nil
	}
	var resp domain.Response
	if err := json.Unmarshal(existing.Response, &resp); err != nil {
		logx.CtxWarnf(ctx, "failed to decode active review response: %v", err)
		return nil
	}
	return &resp
}

func requestFingerprint(req domain.Request) (string, error) {
	data, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("encode review request: %w", err)
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

func queuedSummary(isWebhook bool) string {
	if isWebhook {
		return "review queued (webhook)"
	}
	return "review queued"
}

func queuedMessage(isWebhook bool) string {
	if isWebhook {
		return "review queued via webhook"
	}
	return "review queued"
}

func (s *Service) Get(ctx context.Context, reviewID string) (domain.Record, bool, error) {
	review, err := s.reviewRepo.GetByReviewID(ctx, reviewID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return domain.Record{}, false, nil
		}
		return domain.Record{}, false, err
	}

	var req domain.Request
	if err := json.Unmarshal(review.Request, &req); err != nil {
		return domain.Record{}, false, fmt.Errorf("decode review request: %w", err)
	}
	var resp domain.Response
	if err := json.Unmarshal(review.Response, &resp); err != nil {
		return domain.Record{}, false, fmt.Errorf("decode review response: %w", err)
	}

	if len(review.RawOCR) > 0 && resp.Status == "success" {
		result := domain.Result{
			OCR:      resp.Report.OCR,
			Scope:    resp.Report.Scope,
			Warnings: resp.Report.Warnings,
			Findings: resp.Findings,
		}
		resp.Analysis = BuildAnalysisFromRaw(json.RawMessage(review.RawOCR), result)
	}

	return domain.Record{
		ID:                 review.ReviewID,
		IdempotencyKey:     ptrx.Deref(review.IdempotencyKey, ""),
		RequestFingerprint: review.RequestFingerprint,
		Request:            req,
		Response:           resp,
		RawOCR:             json.RawMessage(review.RawOCR),
		RawOCRInput:        json.RawMessage(review.RawOCRInput),
		CreatedAt:          review.CreatedAt,
		UpdatedAt:          review.UpdatedAt,
	}, true, nil
}

func (s *Service) List(ctx context.Context, options ListOptions) (ListResult, error) {
	if options.Limit <= 0 || options.Limit > 200 {
		options.Limit = 50
	}
	if options.Offset < 0 {
		options.Offset = 0
	}

	page := options.Offset/options.Limit + 1
	result, err := s.reviewRepo.List(ctx, models.ListOptions{
		Status:     options.Status,
		GateStatus: options.GateStatus,
		Provider:   options.Provider,
		Repo:       options.Repo,
		Page:       page,
		PageSize:   options.Limit,
	})
	if err != nil {
		return ListResult{}, err
	}

	items := make([]ListItem, 0, len(result.List))
	for _, review := range result.List {
		var req domain.Request
		if err := json.Unmarshal(review.Request, &req); err != nil {
			logx.CtxWarnf(ctx, "failed to decode review request in list, review_id=%s: %v", review.ReviewID, err)
			continue
		}
		var resp domain.Response
		if err := json.Unmarshal(review.Response, &resp); err != nil {
			logx.CtxWarnf(ctx, "failed to decode review response in list, review_id=%s: %v", review.ReviewID, err)
			continue
		}

		items = append(items, ListItem{
			ReviewID:        review.ReviewID,
			Provider:        req.Provider,
			Repo:            req.Repo,
			Status:          resp.Status,
			Summary:         resp.Summary,
			Gate:            resp.Gate,
			Findings:        len(resp.Findings),
			Score:           resp.Report.Score.Value,
			Grade:           resp.Report.Score.Grade,
			ReportGenerated: resp.Report.Generated,
			Author:          review.Author,
			AuthorName:      review.AuthorName,
			AuthorAvatar:    review.AuthorAvatar,
			CreatedAt:       review.CreatedAt,
			UpdatedAt:       review.UpdatedAt,
		})
	}

	agg, err := s.reviewRepo.AggregateStats(ctx, models.ListOptions{
		Status:     options.Status,
		GateStatus: options.GateStatus,
		Provider:   options.Provider,
		Repo:       options.Repo,
	})
	if err != nil {
		logx.Warn("failed to compute aggregate stats", "error", err)
		agg = &models.AggregateStats{}
	}

	return ListResult{
		Items:      items,
		Total:      int(result.Total),
		Page:       result.Page,
		PageSize:   result.PageSize,
		TotalPages: result.Pages,
		Stats: ListStats{
			Total:       int(result.Total),
			Running:     agg.Running,
			GateFailed:  agg.GateFailed,
			Findings:    agg.Findings,
			TotalTokens: agg.TotalTokens,
		},
	}, nil
}

func (s *Service) Preview(ctx context.Context, req domain.Request) ([]string, error) {
	if err := validate(req); err != nil {
		return nil, err
	}

	p, ok := s.Providers.Get(req.Provider)
	if !ok {
		logx.Errorf("Preview: provider not found: requested=%q", req.Provider)
		return nil, fmt.Errorf("%w: %s", domain.ErrUnsupportedProvider, req.Provider)
	}

	workspace, err := p.Prepare(ctx, req)
	if err != nil {
		return nil, err
	}
	if workspace.Dir != "" {
		defer func() { _ = os.RemoveAll(workspace.Dir) }()
	}

	projectPolicy := models.ProjectPolicy{FailOn: "critical"}
	if s.projectRepo != nil {
		policyRecord, err := s.projectRepo.GetByProviderRepo(ctx, req.Provider, req.Repo)
		if err != nil && !errors.Is(err, repository.ErrNotFound) {
			logx.Warn("failed to get project policy, using default", "error", err)
		}
		if policyRecord != nil {
			projectPolicy = *policyRecord
		}
	}

	var ignorePaths []string
	if len(projectPolicy.IgnorePaths) > 0 {
		if err := json.Unmarshal(projectPolicy.IgnorePaths, &ignorePaths); err != nil {
			logx.Warn("failed to decode ignore_paths", "error", err)
		}
	}
	if len(ignorePaths) > 0 {
		workspace, err = ApplyProjectScope(ctx, workspace, ignorePaths)
		if err != nil {
			return nil, err
		}
	}

	if s.globalRulesRepo != nil {
		rulePath, cleanup, ruleErr := ResolveRuleFile(ctx, projectPolicy, s.rulePackageRepo, s.globalRulesRepo)
		if ruleErr != nil {
			logx.Warn("failed to resolve rule file", "error", ruleErr)
		} else if rulePath != "" {
			workspace.RulePath = rulePath
			if cleanup != nil {
				defer cleanup()
			}
		}
	}

	if s.PreviewRunner == nil {
		return nil, fmt.Errorf("preview feature not configured")
	}
	result, err := s.PreviewRunner.Preview(ctx, workspace)
	if err != nil {
		return nil, err
	}
	return result.Files, nil
}

func (s *Service) Scan(ctx context.Context, req domain.ScanRequest) (*domain.ScanResult, error) {
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
	if s.ScanRunner == nil {
		return nil, fmt.Errorf("scan feature not configured")
	}

	if req.Preview {
		return s.scanExecute(ctx, req)
	}

	return s.scanAsync(ctx, req)
}

func (s *Service) scanAsync(ctx context.Context, req domain.ScanRequest) (*domain.ScanResult, error) {
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

	reviewRow := &models.Review{
		ReviewID:           id,
		RequestFingerprint: fingerprint,
		Provider:           providerName,
		Repo:               repo,
		Status:             StatusQueued,
		BaseSHA:            head,
		HeadSHA:            head,
		Summary:            queuedResp.Summary,
		Request:            mustMarshal(scanReq),
		Response:           mustMarshal(queuedResp),
	}
	if err := s.reviewRepo.Create(ctx, reviewRow); err != nil {
		return nil, err
	}

	var reporter progress.ProgressReporter
	if s.progressManager != nil {
		tracker := s.progressManager.CreateTracker(id, id, 1)
		reporter = tracker
	}

	s.inFlight.Add(1)
	async.GoSafe(func() {
		defer s.inFlight.Done()
		defer func() {
			if s.progressManager != nil {
				s.progressManager.RemoveTracker(id)
			}
		}()

		if s.concurrencySem != nil {
			s.concurrencySem <- struct{}{}
			defer func() { <-s.concurrencySem }()
		}

		s.runScanBackground(id, req, reporter)
	})

	return &domain.ScanResult{ReviewID: id}, nil
}

func (s *Service) runScanBackground(reviewID string, req domain.ScanRequest, reporter progress.ProgressReporter) {
	scanCtx, cancel := context.WithCancel(context.Background())
	s.scanCancels.Store(reviewID, cancel)
	defer func() {
		cancel()
		s.scanCancels.Delete(reviewID)
	}()
	if reporter != nil {
		scanCtx = progress.WithProgressReporter(scanCtx, reporter)
		reporter.UpdatePhase(progress.PhaseContext, "active", 3, "准备仓库工作区...")
	}

	if err := s.updateReviewFields(scanCtx, reviewID, func(review *models.Review) {
		var resp domain.Response
		if err := json.Unmarshal(review.Response, &resp); err != nil {
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
		review.Response = mustMarshal(resp)
		review.UpdatedAt = time.Now().UTC()
	}); err != nil {
		logx.Errorf("failed to update scan status to running: review_id=%s: %v", reviewID, err)
	}

	scanResult, projectPolicy, err := s.scanExecuteWithPolicy(scanCtx, reviewID, req)
	if err != nil {
		logx.Errorf("async scan failed: review_id=%s: %v", reviewID, err)
		s.recordScanFailure(scanCtx, reviewID, err)
		if tracker, ok := reporter.(*progress.ProgressTracker); ok {
			tracker.SetError(err.Error())
		}
		return
	}

	if err := s.persistScanResultUpdate(scanCtx, reviewID, req, projectPolicy, scanResult); err != nil {
		logx.Errorf("failed to persist scan result: review_id=%s: %v", reviewID, err)
		s.recordScanFailure(scanCtx, reviewID, err)
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

func (s *Service) scanExecute(ctx context.Context, req domain.ScanRequest) (*domain.ScanResult, error) {
	scanResult, _, err := s.scanExecuteWithPolicy(ctx, "", req)
	return scanResult, err
}

func (s *Service) scanExecuteWithPolicy(ctx context.Context, reviewID string, req domain.ScanRequest) (*domain.ScanResult, models.ProjectPolicy, error) {
	providerName := strings.TrimSpace(req.Provider)
	repo := strings.TrimSpace(req.Repo)
	head := strings.TrimSpace(req.HeadSHA)

	reporter := progress.ProgressReporterFromContext(ctx)

	p, ok := s.Providers.Get(providerName)
	if !ok {
		logx.Errorf("Scan: provider not found: requested=%q", providerName)
		return nil, models.ProjectPolicy{}, fmt.Errorf("%w: %s", domain.ErrUnsupportedProvider, providerName)
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
		return nil, models.ProjectPolicy{}, err
	}
	if workspace.Dir != "" {
		defer func() { _ = os.RemoveAll(workspace.Dir) }()
	}

	projectPolicy := models.ProjectPolicy{FailOn: "critical"}
	if s.projectRepo != nil {
		policyRecord, err := s.projectRepo.GetByProviderRepo(ctx, providerName, repo)
		if err != nil && !errors.Is(err, repository.ErrNotFound) {
			logx.Warn("failed to get project policy, using default", "error", err)
		}
		if policyRecord != nil {
			projectPolicy = *policyRecord
		}
	}
	workspace.OCR = ProjectPolicyToOCROverrides(projectPolicy)
	if strings.TrimSpace(req.Background) != "" {
		workspace.OCR.Background = req.Background
	}

	if s.globalRulesRepo != nil {
		rulePath, cleanup, ruleErr := ResolveRuleFile(ctx, projectPolicy, s.rulePackageRepo, s.globalRulesRepo)
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
			Binary: "ocr",
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
		if err := s.updateReviewFields(ctx, reviewID, func(review *models.Review) {
			review.RawOCRInput = mustMarshal(ocrInput)
		}); err != nil {
			logx.Warn("failed to persist scan ocr input", "review_id", reviewID, "error", err)
		}
	}

	scanResult, err := s.ScanRunner.Scan(ctx, workspace, req)
	if err != nil {
		return nil, models.ProjectPolicy{}, err
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

func (s *Service) recordScanFailure(ctx context.Context, reviewID string, scanErr error) {
	err := s.updateReviewFields(ctx, reviewID, func(review *models.Review) {
		var resp domain.Response
		if err := json.Unmarshal(review.Response, &resp); err != nil {
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
		review.Response = mustMarshal(resp)
		review.UpdatedAt = time.Now().UTC()
	})
	if err != nil {
		logx.Errorf("failed to record scan failure: review_id=%s: %v", reviewID, err)
	}
}

func (s *Service) persistScanResultUpdate(ctx context.Context, reviewID string, scanReq domain.ScanRequest, projectPolicy models.ProjectPolicy, scanResult *domain.ScanResult) error {
	result := scanResult.Result
	if result == nil {
		return fmt.Errorf("scan result is empty")
	}
	report := BuildReport(result)
	if scanResult.Report != nil {
		report = *scanResult.Report
	}
	analysis := BuildAnalysis(*result)
	if scanResult.Analysis != nil {
		analysis = *scanResult.Analysis
	}

	gateEngine := s.policy
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

	return s.updateReviewFields(ctx, reviewID, func(review *models.Review) {
		review.Status = resp.Status
		review.Score = report.Score.Value
		review.Grade = report.Score.Grade
		review.Summary = resp.Summary
		review.Response = mustMarshal(resp)
		if len(result.RawOCR) > 0 {
			review.RawOCR = datatypes.JSON(result.RawOCR)
		}
		review.UpdatedAt = time.Now().UTC()
	})
}

func (s *Service) updateReviewFields(ctx context.Context, reviewID string, fn func(review *models.Review)) error {
	return s.reviewRepo.UpdateInTransaction(ctx, reviewID, func(review *models.Review) error {
		fn(review)
		return nil
	})
}

// ProjectPolicyToOCROverrides 将项目策略中的 OCR 配置项映射为 review.OCROverrides。
// 作为 scan 路径与 executor 路径的单一实现，消除重复。
func ProjectPolicyToOCROverrides(p models.ProjectPolicy) domain.OCROverrides {
	return domain.OCROverrides{
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

func scanPolicyFindings(findings []domain.Finding) []policy.Finding {
	out := make([]policy.Finding, 0, len(findings))
	for _, finding := range findings {
		out = append(out, policy.Finding{Severity: finding.Severity, Path: finding.Path})
	}
	return out
}

func (s *Service) Requeue(ctx context.Context, reviewID string) (domain.Response, error) {
	// Cancel any orphaned active sessions before re-queuing to avoid
	// "already has an active session" conflicts on the next execution.
	if s.sessionRepo != nil {
		if err := s.sessionRepo.CancelActiveByReviewID(ctx, reviewID); err != nil {
			logx.Warnf("requeue: failed to cancel active sessions for %s: %v", reviewID, err)
		}
	}

	var resultResp domain.Response
	err := s.reviewRepo.UpdateInTransaction(ctx, reviewID, func(review *models.Review) error {
		switch review.Status {
		case StatusQueued, statusRunning:
			return fmt.Errorf("%w: review is %s, only failed or completed reviews can be re-run", ErrInvalidState, review.Status)
		}

		var resp domain.Response
		if err := json.Unmarshal(review.Response, &resp); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}

		resp.Status = StatusQueued
		resp.Summary = "review re-queued"
		resp.Gate = domain.PendingGate()
		resp.Error = ""
		resp.Findings = nil
		resp.Report = domain.Report{}
		resp.Analysis = domain.Analysis{}
		resp.History = append(resp.History, domain.Event{
			Stage:     stageQueued,
			Status:    statusCompleted,
			Message:   "review re-queued by user",
			CreatedAt: time.Now().UTC(),
		})

		review.Status = StatusQueued
		review.Response = mustMarshal(resp)
		review.UpdatedAt = time.Now().UTC()
		resultResp = resp
		return nil
	})
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return domain.Response{}, fmt.Errorf("%w: %s", ErrNotFound, reviewID)
		}
		return domain.Response{}, err
	}
	return resultResp, nil
}

func (s *Service) Cancel(ctx context.Context, reviewID string) (domain.Response, error) {
	if cancelFn, ok := s.scanCancels.LoadAndDelete(reviewID); ok {
		if fn, ok := cancelFn.(context.CancelFunc); ok {
			fn()
		}
	}

	var resultResp domain.Response
	err := s.reviewRepo.UpdateInTransaction(ctx, reviewID, func(review *models.Review) error {
		if review.Status != StatusQueued && review.Status != statusRunning {
			return fmt.Errorf("%w: only queued or running reviews can be cancelled (current: %s)", ErrInvalidState, review.Status)
		}

		var resp domain.Response
		if err := json.Unmarshal(review.Response, &resp); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}

		resp.Status = statusFailed
		resp.Summary = "review cancelled by user"
		resp.Gate = domain.ErrorGate("review cancelled")
		resp.History = append(resp.History, domain.Event{
			Stage:     stageCancelled,
			Status:    statusFailed,
			Message:   "review cancelled by user",
			CreatedAt: time.Now().UTC(),
		})

		review.Status = statusFailed
		review.Response = mustMarshal(resp)
		review.UpdatedAt = time.Now().UTC()
		resultResp = resp
		return nil
	})
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return domain.Response{}, fmt.Errorf("%w: %s", ErrNotFound, reviewID)
		}
		return domain.Response{}, err
	}
	return resultResp, nil
}

func validate(req domain.Request) error {
	if strings.TrimSpace(req.Provider) == "" {
		return mw.NewBadRequestError("provider is required")
	}
	if strings.TrimSpace(req.Repo) == "" {
		return mw.NewBadRequestError("repo is required")
	}

	hasPR := strings.TrimSpace(req.PullRequestID) != "" || strings.TrimSpace(req.MergeRequestIID) != ""

	if !hasPR {
		if strings.TrimSpace(req.BaseSHA) == "" {
			return mw.NewBadRequestError("base_sha is required when pull_request_id or merge_request_iid is not provided")
		}
		if strings.TrimSpace(req.HeadSHA) == "" {
			return mw.NewBadRequestError("head_sha is required when pull_request_id or merge_request_iid is not provided")
		}
		if !gitObjectIDPattern.MatchString(req.BaseSHA) {
			return mw.NewBadRequestError("base_sha must be a 7-64 character hexadecimal Git object ID")
		}
		if !gitObjectIDPattern.MatchString(req.HeadSHA) {
			return mw.NewBadRequestError("head_sha must be a 7-64 character hexadecimal Git object ID")
		}
	}

	if hasPR {
		if req.PullRequestID != "" {
			if _, err := strconv.Atoi(strings.TrimSpace(req.PullRequestID)); err != nil {
				return mw.NewBadRequestError("pull_request_id must be a valid number")
			}
		}
		if req.MergeRequestIID != "" {
			if _, err := strconv.Atoi(strings.TrimSpace(req.MergeRequestIID)); err != nil {
				return mw.NewBadRequestError("merge_request_iid must be a valid number")
			}
		}
	}

	return nil
}

func newReviewID(now time.Time) string {
	b := make([]byte, 4)
	_, _ = crypto_rand.Read(b)
	return fmt.Sprintf("rv_%s_%x", now.Format("20060102"), b)
}
