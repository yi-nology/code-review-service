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

	"github.com/yi-nology/code-review-service/internal/repository"
	"github.com/yi-nology/code-review-service/internal/repository/postgres/models"
	"github.com/yi-nology/code-review-service/internal/service/policy"

	"github.com/oekit/gormx"
	"github.com/oekit/pkg/ptrx"
	"github.com/cloudwego/hertz/pkg/common/hlog"
	"gorm.io/gorm"
)

var gitObjectIDPattern = regexp.MustCompile(`^[0-9a-fA-F]{7,64}$`)

var errIdempotencyKeyTooLong = errors.New("idempotency key too long")

var (
	ErrNotFound      = errors.New("review not found")
	ErrInvalidState  = errors.New("invalid review state")
)

const (
	idempotencyKeyMaxLength = 200

	StatusQueued    = "queued"
	StatusRunning   = "running"
	StatusSuccess   = "success"
	StatusFailed    = "failed"
	StatusCompleted = "completed"

	StageQueued    = "queued"
	StageCancelled = "cancelled"
)

type ServiceConfig struct {
	ReviewRepo      repository.ReviewRepository
	OCRSettingsRepo repository.OCRSettingsRepository
	ProjectRepo     repository.ProjectPolicyRepository
	GlobalRulesRepo repository.GlobalRulesRepository
	ReviewMapRepo   repository.ReviewMapRepository
	Policy          *policy.Engine
	Providers       Providers
	GitProvider     GitProvider
	Runner          Runner
	Publisher       Publisher
	Writeback       WritebackPublisher
	PreviewRunner   PreviewRunner
}

type Service struct {
	reviewRepo      repository.ReviewRepository
	ocrSettingsRepo repository.OCRSettingsRepository
	projectRepo     repository.ProjectPolicyRepository
	globalRulesRepo repository.GlobalRulesRepository
	reviewMapRepo   repository.ReviewMapRepository
	policy          *policy.Engine
	providers       Providers
	gitProvider     GitProvider
	runner          Runner
	publisher       Publisher
	writeback       WritebackPublisher
	previewRunner   PreviewRunner
	inFlight        sync.WaitGroup
}

func NewService(cfg ServiceConfig) *Service {
	return &Service{
		reviewRepo:      cfg.ReviewRepo,
		ocrSettingsRepo: cfg.OCRSettingsRepo,
		projectRepo:     cfg.ProjectRepo,
		globalRulesRepo: cfg.GlobalRulesRepo,
		reviewMapRepo:   cfg.ReviewMapRepo,
		policy:          cfg.Policy,
		providers:       cfg.Providers,
		gitProvider:     cfg.GitProvider,
		runner:          cfg.Runner,
		publisher:       cfg.Publisher,
		writeback:       cfg.Writeback,
		previewRunner:   cfg.PreviewRunner,
	}
}

func (s *Service) Create(ctx context.Context, req Request, idempotencyKey string) (Response, error) {
	return s.createReview(ctx, req, idempotencyKey, false)
}

func (s *Service) CreateFromWebhook(ctx context.Context, req Request, idempotencyKey string) (Response, error) {
	return s.createReview(ctx, req, idempotencyKey, true)
}

func (s *Service) createReview(ctx context.Context, req Request, idempotencyKey string, isWebhook bool) (Response, error) {
	if err := validate(req); err != nil {
		return Response{}, err
	}
	idempotencyKey = strings.TrimSpace(idempotencyKey)
	if len(idempotencyKey) > idempotencyKeyMaxLength {
		return Response{}, errIdempotencyKeyTooLong
	}
	fingerprint, err := requestFingerprint(req)
	if err != nil {
		return Response{}, err
	}

	provider, ok := s.providers.Get(req.Provider)
	if !ok {
		return Response{}, fmt.Errorf("%w: %q", ErrUnsupportedProvider, req.Provider)
	}

	if existing := s.findActiveReview(ctx, req); existing != nil {
		return *existing, nil
	}

	id := newReviewID(time.Now().UTC())
	queued := Response{
		ReviewID: id,
		Status:   StatusQueued,
		Summary:  queuedSummary(isWebhook),
		Gate:     PendingGate(),
		History: []Event{{
			Stage:     StageQueued,
			Status:    StatusCompleted,
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
		Request:            mustMarshal(req),
		Response:           mustMarshal(queued),
	}
	if idempotencyKey != "" {
		review.IdempotencyKey = &idempotencyKey
	}

	if err := s.reviewRepo.Create(ctx, review); err != nil {
		if idempotencyKey != "" && gormx.IsUniqueViolation(err) {
			existing, getErr := s.reviewRepo.GetByidempotencyKey(ctx, idempotencyKey)
			if getErr == nil && existing != nil {
				if existing.RequestFingerprint != fingerprint {
					return Response{}, ErrIdempotencyConflict
				}
				var resp Response
				if err := json.Unmarshal(existing.Response, &resp); err != nil {
					return Response{}, fmt.Errorf("decode existing review response: %w", err)
				}
				return resp, nil
			}
		}
		return Response{}, err
	}

	if !req.Wait {
		return queued, nil
	}

	return s.run(ctx, id, req, provider)
}

func (s *Service) findActiveReview(ctx context.Context, req Request) *Response {
	hasPR := req.PullRequestID != "" || req.MergeRequestIID != ""
	if !hasPR {
		return nil
	}
	existing, err := s.reviewRepo.GetActiveByProviderRepo(ctx, req.Provider, req.Repo)
	if err != nil {
		hlog.CtxErrorf(ctx, "failed to query active review by provider=%s repo=%s: %v", req.Provider, req.Repo, err)
		return nil
	}
	if existing == nil {
		return nil
	}
	var existingReq Request
	if err := json.Unmarshal(existing.Request, &existingReq); err != nil {
		hlog.CtxWarnf(ctx, "failed to decode active review request: %v", err)
		return nil
	}
	if existingReq.PullRequestID != req.PullRequestID || existingReq.MergeRequestIID != req.MergeRequestIID {
		return nil
	}
	var resp Response
	if err := json.Unmarshal(existing.Response, &resp); err != nil {
		hlog.CtxWarnf(ctx, "failed to decode active review response: %v", err)
		return nil
	}
	return &resp
}

func requestFingerprint(req Request) (string, error) {
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

func (s *Service) Get(ctx context.Context, reviewID string) (Record, bool, error) {
	review, err := s.reviewRepo.GetByReviewID(ctx, reviewID)
	if err != nil {
		return Record{}, false, err
	}
	if review == nil {
		return Record{}, false, nil
	}

	var req Request
	if err := json.Unmarshal(review.Request, &req); err != nil {
		return Record{}, false, fmt.Errorf("decode review request: %w", err)
	}
	var resp Response
	if err := json.Unmarshal(review.Response, &resp); err != nil {
		return Record{}, false, fmt.Errorf("decode review response: %w", err)
	}

	if len(review.RawOCR) > 0 && resp.Status == "success" {
		result := Result{
			OCR:      resp.Report.OCR,
			Scope:    resp.Report.Scope,
			Warnings: resp.Report.Warnings,
			Findings: resp.Findings,
		}
		resp.Analysis = BuildAnalysisFromRaw(json.RawMessage(review.RawOCR), result)
	}

	return Record{
		ID:                 review.ReviewID,
		IdempotencyKey:     ptrx.Deref(review.IdempotencyKey, ""),
		RequestFingerprint: review.RequestFingerprint,
		Request:            req,
		Response:           resp,
		RawOCR:             json.RawMessage(review.RawOCR),
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
		Status:   options.Status,
		Provider: options.Provider,
		Repo:     options.Repo,
		Page:     page,
		PageSize: options.Limit,
	})
	if err != nil {
		return ListResult{}, err
	}

	items := make([]ListItem, 0, len(result.List))
	for _, review := range result.List {
		var req Request
		if err := json.Unmarshal(review.Request, &req); err != nil {
			continue
		}
		var resp Response
		if err := json.Unmarshal(review.Response, &resp); err != nil {
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
		Status:   options.Status,
		Provider: options.Provider,
		Repo:     options.Repo,
	})
	if err != nil {
		hlog.Warn("failed to compute aggregate stats", "error", err)
		agg = &models.AggregateStats{}
	}

	return ListResult{
		Items:      items,
		Total:      int(result.Total),
		Page:       result.Page,
		PageSize:   result.PageSize,
		TotalPages: result.Pages,
		Stats: ListStats{
			Total:      int(result.Total),
			Running:    agg.Running,
			GateFailed: agg.GateFailed,
			Findings:   agg.Findings,
		},
	}, nil
}

func (s *Service) Preview(ctx context.Context, req Request) ([]string, error) {
	if err := validate(req); err != nil {
		return nil, err
	}

	p, ok := s.providers.Get(req.Provider)
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedProvider, req.Provider)
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
		if err == nil && policyRecord != nil {
			projectPolicy = *policyRecord
		}
	}

	var ignorePaths []string
	if len(projectPolicy.IgnorePaths) > 0 {
		if err := json.Unmarshal(projectPolicy.IgnorePaths, &ignorePaths); err != nil {
			hlog.Warn("failed to decode ignore_paths", "error", err)
		}
	}
	if len(ignorePaths) > 0 {
		workspace, err = applyProjectScope(ctx, workspace, ignorePaths)
		if err != nil {
			return nil, err
		}
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

	if s.previewRunner == nil {
		return nil, fmt.Errorf("preview feature not configured")
	}
	result, err := s.previewRunner.Preview(ctx, workspace)
	if err != nil {
		return nil, err
	}
	return result.Files, nil
}

func (s *Service) Requeue(ctx context.Context, reviewID string) (Response, error) {
	var resultResp Response
	err := s.reviewRepo.UpdateInTransaction(ctx, reviewID, func(review *models.Review) error {
		switch review.Status {
		case StatusQueued, StatusRunning:
			return fmt.Errorf("%w: review is %s, only failed or completed reviews can be re-run", ErrInvalidState, review.Status)
		}

		var resp Response
		if err := json.Unmarshal(review.Response, &resp); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}

		resp.Status = StatusQueued
		resp.Summary = "review re-queued"
		resp.Gate = PendingGate()
		resp.Error = ""
		resp.Findings = nil
		resp.Report = Report{}
		resp.Analysis = Analysis{}
		resp.History = append(resp.History, Event{
			Stage:     StageQueued,
			Status:    StatusCompleted,
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
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return Response{}, fmt.Errorf("%w: %s", ErrNotFound, reviewID)
		}
		return Response{}, err
	}
	return resultResp, nil
}

func (s *Service) Cancel(ctx context.Context, reviewID string) (Response, error) {
	var resultResp Response
	err := s.reviewRepo.UpdateInTransaction(ctx, reviewID, func(review *models.Review) error {
		if review.Status != StatusQueued {
			return fmt.Errorf("%w: only queued reviews can be cancelled (current: %s)", ErrInvalidState, review.Status)
		}

		var resp Response
		if err := json.Unmarshal(review.Response, &resp); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}

		resp.Status = StatusFailed
		resp.Summary = "review cancelled by user"
		resp.Gate = ErrorGate("review cancelled")
		resp.History = append(resp.History, Event{
			Stage:     StageCancelled,
			Status:    StatusFailed,
			Message:   "review cancelled by user",
			CreatedAt: time.Now().UTC(),
		})

		review.Status = StatusFailed
		review.Response = mustMarshal(resp)
		review.UpdatedAt = time.Now().UTC()
		resultResp = resp
		return nil
	})
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return Response{}, fmt.Errorf("%w: %s", ErrNotFound, reviewID)
		}
		return Response{}, err
	}
	return resultResp, nil
}

func validate(req Request) error {
	if strings.TrimSpace(req.Provider) == "" {
		return errors.New("provider is required")
	}
	if strings.TrimSpace(req.Repo) == "" {
		return errors.New("repo is required")
	}

	hasPR := strings.TrimSpace(req.PullRequestID) != "" || strings.TrimSpace(req.MergeRequestIID) != ""

	if !hasPR {
		if strings.TrimSpace(req.BaseSHA) == "" {
			return errors.New("base_sha is required when pull_request_id or merge_request_iid is not provided")
		}
		if strings.TrimSpace(req.HeadSHA) == "" {
			return errors.New("head_sha is required when pull_request_id or merge_request_iid is not provided")
		}
		if !gitObjectIDPattern.MatchString(req.BaseSHA) {
			return errors.New("base_sha must be a 7-64 character hexadecimal Git object ID")
		}
		if !gitObjectIDPattern.MatchString(req.HeadSHA) {
			return errors.New("head_sha must be a 7-64 character hexadecimal Git object ID")
		}
	}

	if hasPR {
		if req.PullRequestID != "" {
			if _, err := strconv.Atoi(strings.TrimSpace(req.PullRequestID)); err != nil {
				return errors.New("pull_request_id must be a valid number")
			}
		}
		if req.MergeRequestIID != "" {
			if _, err := strconv.Atoi(strings.TrimSpace(req.MergeRequestIID)); err != nil {
				return errors.New("merge_request_iid must be a valid number")
			}
		}
	}

	return nil
}

func newReviewID(now time.Time) string {
	return fmt.Sprintf("rv_%s_%d", now.Format("20060102"), now.UnixNano())
}
