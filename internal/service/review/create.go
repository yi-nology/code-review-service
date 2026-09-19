package review

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/yi-nology/code-review-service/internal/argusclient"
	"github.com/yi-nology/code-review-service/internal/repository/repoerr"
	"github.com/yi-nology/code-review-service/internal/service/review/domain"

	bizerrors "github.com/oekit/pkg/errors"

	"github.com/oekit/gormx" //lint:ignore SA1019 PageResponse 未废弃，上游包级别注释误标
	"github.com/oekit/logx"
)

// create.go：评审创建路径（幂等、引擎分流、入队）。
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
	// 引擎解析（请求显式 > 项目策略 review_engine > 默认 OCR）在指纹计算前完成：
	// 生效引擎随 Request 持久化，worker/执行器据此分流。
	if err := s.resolveEngine(ctx, &req); err != nil {
		return domain.Response{}, err
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

	id := NewReviewID(time.Now().UTC())
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

// resolveEngine 解析生效审查引擎与门禁模式：请求显式指定 > 项目策略 > 默认
// （引擎 OCR、门禁 auto）。结果写回 req.Engine / req.GateMode（随 Request 持久化，
// worker/执行器/回报收敛据此分流与决定 PR 门禁红绿）；engine=argus 但未配置
// argus 对接时 fail-fast 拒绝。
func (s *Service) resolveEngine(ctx context.Context, req *domain.Request) error {
	req.Engine = strings.ToLower(strings.TrimSpace(req.Engine))
	// gate_mode 不随请求上送，一律从项目策略解析（旧记录无该字段时保持 auto）。
	if s.projectRepo != nil {
		if p, err := s.projectRepo.GetByProviderRepo(ctx, req.Provider, req.Repo); err == nil && p != nil {
			if req.Engine == "" {
				req.Engine = strings.ToLower(strings.TrimSpace(p.ReviewEngine))
			}
			req.GateMode = strings.ToLower(strings.TrimSpace(p.GateMode))
		}
	}
	if req.Engine == "" {
		req.Engine = domain.EngineOCR
	}
	if req.Engine == domain.EngineArgus {
		if !s.argusEnabled {
			return bizerrors.NewBadRequestError("argus 引擎未启用（argus.bot_url 未配置）")
		}
		// argus 侧只支持 gitea/github/gitlab：gerrit/gitcode/tencent_code 的项目
		// 策略误配 argus 时创建期 fail-fast（否则任务到 argus 侧才白白失败）。
		if !argusclient.SupportsPlatform(req.Provider) {
			return bizerrors.NewBadRequestError(
				"argus 引擎不支持平台 " + req.Provider + "（仅支持 gitea/github/gitlab），请调整项目策略 review_engine")
		}
	}
	return nil
}

// requestFingerprint 对请求做幂等指纹。engine 参与指纹：同一 PR 换引擎重审
// 是不同的请求，不应命中 idempotency-key 冲突。
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

func validate(req domain.Request) error {
	if strings.TrimSpace(req.Provider) == "" {
		return bizerrors.NewBadRequestError("provider is required")
	}
	if strings.TrimSpace(req.Repo) == "" {
		return bizerrors.NewBadRequestError("repo is required")
	}

	hasPR := strings.TrimSpace(req.PullRequestID) != "" || strings.TrimSpace(req.MergeRequestIID) != ""
	hasCommit := strings.TrimSpace(req.CommitSHA) != ""

	if hasCommit {
		// 单 commit 评审模式（OCR v1.9.5+ --commit）：CommitSHA 与 PR/MR 互斥。
		if hasPR {
			return bizerrors.NewBadRequestError("commit_sha cannot be used together with pull_request_id or merge_request_iid")
		}
		if !gitObjectIDPattern.MatchString(req.CommitSHA) {
			return bizerrors.NewBadRequestError("commit_sha must be a 7-64 character hexadecimal Git object ID")
		}
	} else if !hasPR {
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

	switch strings.TrimSpace(req.Engine) {
	case "", domain.EngineOCR, domain.EngineArgus:
	default:
		return bizerrors.NewBadRequestError("engine 仅支持 ocr、argus")
	}
	// argus 引擎按 PR 维度审查，单 commit 模式（--commit）是 OCR 专属能力。
	if req.Engine == domain.EngineArgus && hasCommit {
		return bizerrors.NewBadRequestError("engine=argus 不支持 commit_sha 单 commit 模式")
	}
	// task_kind 是 argus 专属：OCR 请求携带即拒绝（避免静默丢弃造成误解）。
	if kind := strings.TrimSpace(req.TaskKind); kind != "" {
		if req.Engine != domain.EngineArgus {
			return bizerrors.NewBadRequestError("task_kind 仅 engine=argus 支持")
		}
		switch domain.NormalizeTaskKind(kind) {
		case domain.TaskKindCodeReview, domain.TaskKindRequirementCompliance, domain.TaskKindVisualCheck:
		default:
			return bizerrors.NewBadRequestError(
				"task_kind 仅支持 code_review、requirement_compliance、visual_check")
		}
	}

	return nil
}

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
