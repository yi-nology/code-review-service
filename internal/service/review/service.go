package review

import (
	"context"
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

	"github.com/yi-nology/code-review-service/internal/domain/projectpolicy"
	"github.com/yi-nology/code-review-service/internal/repository"
	"github.com/yi-nology/code-review-service/internal/repository/repoerr"
	"github.com/yi-nology/code-review-service/internal/service/policy"
	"github.com/yi-nology/code-review-service/internal/service/progress"
	gitprovider "github.com/yi-nology/code-review-service/internal/service/provider"
	"github.com/yi-nology/code-review-service/internal/service/review/domain"
	"github.com/yi-nology/code-review-service/internal/service/types"

	"github.com/oekit/concurrency/async"
	bizerrors "github.com/oekit/pkg/errors"

	"github.com/oekit/gormx" //lint:ignore SA1019 PageResponse 未废弃，上游包级别注释误标
	"github.com/oekit/logx"
)

var gitObjectIDPattern = regexp.MustCompile(`^[0-9a-fA-F]{7,64}$`)

var errIdempotencyKeyTooLong = bizerrors.NewBadRequestError("idempotency key too long")

var (
	ErrNotFound     = repoerr.ErrNotFound
	ErrInvalidState = repoerr.ErrInvalidState
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
	// Concurrency 限制同时进行的 review 数（worker 与 scan 共享）。
	// <=0 表示不限制。在构造期一次性创建信号量，避免运行期懒初始化导致的数据竞争。
	Concurrency int
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
	s := &Service{
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
	// 构造期一次性创建并发信号量，消除运行期懒初始化的数据竞争。
	// nil 表示不限制（Concurrency<=0）。
	if cfg.Concurrency > 0 {
		s.concurrencySem = make(chan struct{}, cfg.Concurrency)
		logx.Infof("review concurrency limit=%d (worker & scan shared)", cfg.Concurrency)
	}
	return s
}

// acquireConcurrency 获取并发槽，支持 ctx 取消。
// 信号量为 nil（未配置限制）时直接放行。
// 返回 false 表示等待期间 ctx 已取消，调用方应中止并清理。
func (s *Service) acquireConcurrency(ctx context.Context) bool {
	if s.concurrencySem == nil {
		return true
	}
	select {
	case s.concurrencySem <- struct{}{}:
		return true
	case <-ctx.Done():
		return false
	}
}

func (s *Service) releaseConcurrency() {
	if s.concurrencySem != nil {
		<-s.concurrencySem
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

	review := &domain.Record{
		ID:                 id,
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
		Request:            req,
		Response:           queued,
	}
	if idempotencyKey != "" {
		review.IdempotencyKey = idempotencyKey
	}

	if err := s.reviewRepo.Create(ctx, review); err != nil {
		if idempotencyKey != "" && gormx.IsUniqueViolation(err) {
			existing, getErr := s.reviewRepo.GetByIdempotencyKey(ctx, idempotencyKey)
			if getErr != nil && !errors.Is(getErr, repoerr.ErrNotFound) {
				return domain.Response{}, fmt.Errorf("idempotency recovery: %w", getErr)
			}
			if existing != nil {
				if existing.RequestFingerprint != fingerprint {
					return domain.Response{}, domain.ErrIdempotencyConflict
				}
				return existing.Response, nil
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

// findActiveReview 查找同一 (provider, repo, PR/MR) 的 active review。
//
// PR-02 修复：把 PR/MR 过滤推下 SQL（见 GetActiveByProviderRepo 实现），
// 避免原先 unmarshal 整条 response JSONB 的开销与 TOCTOU 风险。
// 仅在有 PR/MR 时才查找；scan 路径（两者都为空）返回 nil。
func (s *Service) findActiveReview(ctx context.Context, req domain.Request) *domain.Response {
	if req.PullRequestID == "" && req.MergeRequestIID == "" {
		return nil
	}
	existing, err := s.reviewRepo.GetActiveByProviderRepo(ctx, req.Provider, req.Repo, req.PullRequestID, req.MergeRequestIID)
	if err != nil {
		if errors.Is(err, repoerr.ErrNotFound) {
			return nil
		}
		logx.CtxErrorf(ctx, "failed to query active review by provider=%s repo=%s: %v", req.Provider, req.Repo, err)
		return nil
	}
	resp := existing.Response
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
		if errors.Is(err, repoerr.ErrNotFound) {
			return domain.Record{}, false, nil
		}
		return domain.Record{}, false, err
	}

	// review 已是 domain.Record（Request/Response 强类型），仅按需补 Analysis。
	if len(review.RawOCR) > 0 && review.Response.Status == "success" {
		result := domain.Result{
			OCR:      review.Response.Report.OCR,
			Scope:    review.Response.Report.Scope,
			Warnings: review.Response.Report.Warnings,
			Findings: review.Response.Findings,
		}
		review.Response.Analysis = BuildAnalysisFromRaw(json.RawMessage(review.RawOCR), result)
	}

	return *review, true, nil
}

func (s *Service) List(ctx context.Context, options ListOptions) (ListResult, error) {
	if options.Limit <= 0 || options.Limit > 200 {
		options.Limit = 50
	}
	if options.Offset < 0 {
		options.Offset = 0
	}

	page := options.Offset/options.Limit + 1
	result, err := s.reviewRepo.List(ctx, domain.ListOptions{
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
		// review 已是 domain.Record，Request/Response 强类型，无需 unmarshal。
		items = append(items, ListItem{
			ReviewID:        review.ID,
			Provider:        review.Request.Provider,
			Repo:            review.Request.Repo,
			Status:          review.Response.Status,
			Summary:         review.Response.Summary,
			Gate:            review.Response.Gate,
			Findings:        len(review.Response.Findings),
			Score:           review.Response.Report.Score.Value,
			Grade:           review.Response.Report.Score.Grade,
			ReportGenerated: review.Response.Report.Generated,
			Author:          review.Author,
			AuthorName:      review.AuthorName,
			AuthorAvatar:    review.AuthorAvatar,
			CreatedAt:       review.CreatedAt,
			UpdatedAt:       review.UpdatedAt,
		})
	}

	agg, err := s.reviewRepo.AggregateStats(ctx, domain.ListOptions{
		Status:     options.Status,
		GateStatus: options.GateStatus,
		Provider:   options.Provider,
		Repo:       options.Repo,
	})
	if err != nil {
		logx.Warn("failed to compute aggregate stats", "error", err)
		agg = &domain.AggregateStats{}
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

	projectPolicy := projectpolicy.ProjectPolicy{FailOn: "critical"}
	if s.projectRepo != nil {
		policyRecord, err := s.projectRepo.GetByProviderRepo(ctx, req.Provider, req.Repo)
		if err != nil && !errors.Is(err, repoerr.ErrNotFound) {
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

		// 获取并发槽：可被 ctx 取消，避免停机时阻塞发送导致 Shutdown 超时死锁。
		if !s.acquireConcurrency(ctx) {
			logx.Warnf("scan %s cancelled while waiting for concurrency slot", id)
			return
		}
		defer s.releaseConcurrency()

		// PR-03 修复：派生自 ctx，使得 HTTP 请求被取消时 scan goroutine 也能收到信号。
		// 旧的 context.WithCancel(context.Background()) 与请求 lifecycle 完全脱钩，
		// 只能依赖外部 Cancel() 显式调用。
		scanCtx, cancel := context.WithCancel(ctx)
		s.scanCancels.Store(id, cancel)
		defer func() {
			cancel()
			s.scanCancels.Delete(id)
		}()

		s.runScanBackground(scanCtx, id, req, reporter)
	})

	return &domain.ScanResult{ReviewID: id}, nil
}

func (s *Service) runScanBackground(scanCtx context.Context, reviewID string, req domain.ScanRequest, reporter progress.ProgressReporter) {
	if reporter != nil {
		scanCtx = progress.WithProgressReporter(scanCtx, reporter)
		reporter.UpdatePhase(progress.PhaseContext, "active", 3, "准备仓库工作区...")
	}

	if err := s.updateReviewFields(scanCtx, reviewID, func(review *domain.Record) {
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

func (s *Service) scanExecuteWithPolicy(ctx context.Context, reviewID string, req domain.ScanRequest) (*domain.ScanResult, projectpolicy.ProjectPolicy, error) {
	providerName := strings.TrimSpace(req.Provider)
	repo := strings.TrimSpace(req.Repo)
	head := strings.TrimSpace(req.HeadSHA)

	reporter := progress.ProgressReporterFromContext(ctx)

	p, ok := s.Providers.Get(providerName)
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

	projectPolicy := projectpolicy.ProjectPolicy{FailOn: "critical"}
	if s.projectRepo != nil {
		policyRecord, err := s.projectRepo.GetByProviderRepo(ctx, providerName, repo)
		if err != nil && !errors.Is(err, repoerr.ErrNotFound) {
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
		if err := s.updateReviewFields(ctx, reviewID, func(review *domain.Record) {
			rawOCRInputJSON, _ := json.Marshal(ocrInput)
			review.RawOCRInput = rawOCRInputJSON
		}); err != nil {
			logx.Warn("failed to persist scan ocr input", "review_id", reviewID, "error", err)
		}
	}

	scanResult, err := s.ScanRunner.Scan(ctx, workspace, req)
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

func (s *Service) recordScanFailure(ctx context.Context, reviewID string, scanErr error) {
	err := s.updateReviewFields(ctx, reviewID, func(review *domain.Record) {
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

func (s *Service) persistScanResultUpdate(ctx context.Context, reviewID string, scanReq domain.ScanRequest, projectPolicy projectpolicy.ProjectPolicy, scanResult *domain.ScanResult) error {
	result := scanResult.Result
	if result == nil {
		return fmt.Errorf("scan result is empty")
	}
	// 在 gate/report/writeback 消费 findings 之前，统一计算跨 review 稳定指纹。
	types.ComputeFingerprints(result.Findings)
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

	return s.updateReviewFields(ctx, reviewID, func(review *domain.Record) {
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

func (s *Service) updateReviewFields(ctx context.Context, reviewID string, fn func(review *domain.Record)) error {
	return s.reviewRepo.UpdateInTransaction(ctx, reviewID, func(review *domain.Record) error {
		fn(review)
		return nil
	})
}

// ProjectPolicyToOCROverrides 将项目策略中的 OCR 配置项映射为 review.OCROverrides。
// 作为 scan 路径与 executor 路径的单一实现，消除重复。
func ProjectPolicyToOCROverrides(p projectpolicy.ProjectPolicy) domain.OCROverrides {
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

// MergeRequirementsIntoBackground 把需求/spec 追加进 OCR --background 上下文。
//
// OCR 的 --background 让审查 agent 拿到 diff 之外的背景（需求、约束、验收标准），
// 官方明确建议「有 PR/需求上下文时始终传 --background，质量提升显著」。
// 此前 req.Requirements 仅在 workflow 多 agent 路径（嵌入 LLM prompt）里被使用，
// 单 runner 路径直接调 OCR 时丢失了需求上下文 —— 本函数补上这个缺口。
//
// 合并规则：项目策略的 Background 在前，requirements 追加其后（已有内容则换行分隔），
// 两者都保留，让 OCR 同时看到项目级背景和本次评审需求。
func MergeRequirementsIntoBackground(existing string, requirements []string) string {
	var parts []string
	if b := strings.TrimSpace(existing); b != "" {
		parts = append(parts, b)
	}
	for _, r := range requirements {
		if r = strings.TrimSpace(r); r != "" {
			parts = append(parts, r)
		}
	}
	return strings.Join(parts, "\n")
}

func scanPolicyFindings(findings []domain.Finding) []policy.Finding {
	out := make([]policy.Finding, 0, len(findings))
	for _, finding := range findings {
		out = append(out, policy.Finding{Severity: finding.Severity, Path: finding.Path})
	}
	return out
}

func validate(req domain.Request) error {
	if strings.TrimSpace(req.Provider) == "" {
		return bizerrors.NewBadRequestError("provider is required")
	}
	if strings.TrimSpace(req.Repo) == "" {
		return bizerrors.NewBadRequestError("repo is required")
	}

	hasPR := strings.TrimSpace(req.PullRequestID) != "" || strings.TrimSpace(req.MergeRequestIID) != ""

	if !hasPR {
		if strings.TrimSpace(req.BaseSHA) == "" {
			return bizerrors.NewBadRequestError("base_sha is required when pull_request_id or merge_request_iid is not provided")
		}
		if strings.TrimSpace(req.HeadSHA) == "" {
			return bizerrors.NewBadRequestError("head_sha is required when pull_request_id or merge_request_iid is not provided")
		}
		if !gitObjectIDPattern.MatchString(req.BaseSHA) {
			return bizerrors.NewBadRequestError("base_sha must be a 7-64 character hexadecimal Git object ID")
		}
		if !gitObjectIDPattern.MatchString(req.HeadSHA) {
			return bizerrors.NewBadRequestError("head_sha must be a 7-64 character hexadecimal Git object ID")
		}
	}

	if hasPR {
		if req.PullRequestID != "" {
			if _, err := strconv.Atoi(strings.TrimSpace(req.PullRequestID)); err != nil {
				return bizerrors.NewBadRequestError("pull_request_id must be a valid number")
			}
		}
		if req.MergeRequestIID != "" {
			if _, err := strconv.Atoi(strings.TrimSpace(req.MergeRequestIID)); err != nil {
				return bizerrors.NewBadRequestError("merge_request_iid must be a valid number")
			}
		}
	}

	return nil
}

// newReviewID 别名由 id.go 提供，保留 service.go 内部调用点不动。
