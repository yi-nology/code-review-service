package review_service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/yi-nology/code-review-service/biz/handler/common"
	modelPlatform "github.com/yi-nology/code-review-service/biz/model/platform"
	modelReview "github.com/yi-nology/code-review-service/biz/model/review"
	svcApp "github.com/yi-nology/code-review-service/internal/app"
	"github.com/yi-nology/code-review-service/internal/service/review"
	"github.com/yi-nology/code-review-service/internal/service/review/domain"
	reviewenrich "github.com/yi-nology/code-review-service/internal/service/reviewenrich"
	"github.com/yi-nology/code-review-service/internal/service/suggestion"

	"github.com/oekit/pkg/convert"
	"github.com/oekit/pkg/pagination"
	"github.com/oekit/pkg/ptrx"
	"github.com/oekit/pkg/slicex"
	"github.com/oekit/pkg/stringx"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	provider_sdk "github.com/yi-nology/git-platform-sdk/provider"
)

// review_service.go：评审/扫描/平台浏览端点（方法 + 包级一行委托）。
type ReviewHandler struct {
	reviewSvc     svcApp.ReviewService
	platform      svcApp.PlatformReader
	crWriter      svcApp.PlatformCRWriter
	enrich        *reviewenrich.Service
	suggestionSvc *suggestion.Service
}

func InitHandler(svc svcApp.ReviewService, platform svcApp.PlatformReader, crWriter svcApp.PlatformCRWriter, enrich *reviewenrich.Service, suggestionSvc *suggestion.Service) {
	handlerInst = &ReviewHandler{
		reviewSvc:     svc,
		platform:      platform,
		crWriter:      crWriter,
		enrich:        enrich,
		suggestionSvc: suggestionSvc,
	}
}

func (h *ReviewHandler) createReview(ctx context.Context, c *app.RequestContext) {
	req, ok := common.Bind[modelReview.CreateReviewReq](c)
	if !ok {
		return
	}

	svcReq := toServiceRequest(req)
	h.enrich.FillAuthor(ctx, &svcReq)

	idempotencyKey := string(c.GetHeader("Idempotency-Key"))
	resp, err := h.reviewSvc.Create(ctx, svcReq, idempotencyKey)
	if err != nil {
		common.Fail(ctx, c, mapReviewError(err))
		return
	}

	converted := convertResponse(resp)
	apiResp := &modelReview.CreateReviewResp{Review: &converted}

	status := consts.StatusOK
	if resp.Status == review.StatusQueued {
		status = consts.StatusAccepted
	}
	c.JSON(status, apiResp)
}

// CreateReview 包级入口（biz/router 生成代码引用）；一行委托到方法，测试可直接构造 Handler。
func CreateReview(ctx context.Context, c *app.RequestContext) { handlerInst.createReview(ctx, c) }

func (h *ReviewHandler) getReview(ctx context.Context, c *app.RequestContext) {
	// 手动提取 :id URL 参数（thrift 生成的 struct 无法自动绑定 URL 参数）
	reviewID := c.Param("id")
	if reviewID == "" {
		common.Fail(ctx, c, common.BadRequest("review id is required"))
		return
	}

	record, ok, err := h.reviewSvc.Get(ctx, reviewID)
	if err != nil {
		common.Fail(ctx, c, err)
		return
	}
	if !ok {
		common.Fail(ctx, c, common.NotFound("review not found"))
		return
	}

	detail := convertDetail(record)
	common.Success(c, &modelReview.GetReviewResp{Review: &detail})
}

// GetReview 包级入口（biz/router 生成代码引用）；一行委托到方法，测试可直接构造 Handler。
func GetReview(ctx context.Context, c *app.RequestContext) { handlerInst.getReview(ctx, c) }

func (h *ReviewHandler) listReviews(ctx context.Context, c *app.RequestContext) {
	req, ok := common.Bind[modelReview.ListReviewsReq](c)
	if !ok {
		return
	}

	// thrift 生成的 struct 没有 query tag，BindAndValidate 无法绑定 GET 查询参数。
	// page/page_size 需要从 query string 手动读取，与 sort_by/gate_status 一致。
	pageStr := string(c.Query("page"))
	if pageStr == "" {
		pageStr = convert.Int64ToStr(ptrx.Deref(req.Page, 0))
	}
	pageSizeStr := string(c.Query("page_size"))
	if pageSizeStr == "" {
		pageSizeStr = convert.Int64ToStr(ptrx.Deref(req.PageSize, 0))
	}
	page, pageSize := common.PageParams(pageStr, pageSizeStr)

	result, err := h.reviewSvc.List(ctx, review.ListOptions{
		Status:     strings.TrimSpace(ptrx.Deref(req.Status, "")),
		GateStatus: strings.TrimSpace(string(c.Query("gate_status"))),
		Provider:   strings.TrimSpace(ptrx.Deref(req.Provider, "")),
		Repo:       strings.TrimSpace(ptrx.Deref(req.Repo, "")),
		SortBy:     strings.TrimSpace(string(c.Query("sort_by"))),
		SortOrder:  strings.TrimSpace(string(c.Query("sort_order"))),
		Limit:      pageSize,
		Offset:     (page - 1) * pageSize,
	})
	if err != nil {
		common.Fail(ctx, c, err)
		return
	}

	items := slicex.Map(result.Items, func(item review.ListItem) *modelReview.ListItem {
		return &modelReview.ListItem{
			ReviewID:        item.ReviewID,
			Provider:        item.Provider,
			Repo:            item.Repo,
			Status:          item.Status,
			Summary:         item.Summary,
			Gate:            convertGate(item.Gate),
			Findings:        int64(item.Findings),
			Score:           int64(item.Score),
			Grade:           common.StrPtr(item.Grade),
			ReportGenerated: item.ReportGenerated,
			Author:          common.StrPtr(item.Author),
			AuthorName:      common.StrPtr(item.AuthorName),
			AuthorAvatar:    common.StrPtr(item.AuthorAvatar),
			RetryCount:      ptrx.Of(int64(item.RetryCount)),
			CreatedAt:       common.StrPtr(common.FormatTime(item.CreatedAt)),
			UpdatedAt:       common.StrPtr(common.FormatTime(item.UpdatedAt)),
		}
	})

	common.Success(c, &modelReview.ListReviewsResp{
		Items:      items,
		Total:      int64(result.Total),
		Page:       int64(result.Page),
		PageSize:   int64(result.PageSize),
		TotalPages: int64(result.TotalPages),
		Stats: &modelReview.ListStats{
			Total:       int64(result.Stats.Total),
			Running:     int64(result.Stats.Running),
			GateFailed:  int64(result.Stats.GateFailed),
			Findings:    int64(result.Stats.Findings),
			TotalTokens: ptrx.Of(result.Stats.TotalTokens),
		},
	})
}

// ListReviews 包级入口（biz/router 生成代码引用）；一行委托到方法，测试可直接构造 Handler。
func ListReviews(ctx context.Context, c *app.RequestContext) { handlerInst.listReviews(ctx, c) }

func (h *ReviewHandler) previewReview(ctx context.Context, c *app.RequestContext) {
	req, ok := common.Bind[modelReview.PreviewReq](c)
	if !ok {
		return
	}

	svcReq := domain.Request{
		Provider:        req.Provider,
		Repo:            req.Repo,
		ProjectID:       ptrx.Deref(req.ProjectID, ""),
		PullRequestID:   ptrx.Deref(req.PullRequestID, ""),
		MergeRequestIID: ptrx.Deref(req.MergeRequestIid, ""),
		BaseSHA:         ptrx.Deref(req.BaseSha, ""),
		HeadSHA:         ptrx.Deref(req.HeadSha, ""),
	}

	files, err := h.reviewSvc.Preview(ctx, svcReq)
	if err != nil {
		common.Fail(ctx, c, mapReviewError(err))
		return
	}

	common.Success(c, &modelReview.PreviewResp{Files: files})
}

// PreviewReview 包级入口（biz/router 生成代码引用）；一行委托到方法，测试可直接构造 Handler。
func PreviewReview(ctx context.Context, c *app.RequestContext) { handlerInst.previewReview(ctx, c) }

type scanRepositoryReq struct {
	Provider        string   `json:"provider"`
	Repo            string   `json:"repo"`
	ProjectID       string   `json:"project_id,omitempty"`
	HeadSHA         string   `json:"head_sha"`
	Paths           []string `json:"paths,omitempty"`
	Exclude         []string `json:"exclude,omitempty"`
	Background      string   `json:"background,omitempty"`
	MaxTokensBudget int      `json:"max_tokens_budget,omitempty"`
	Batch           string   `json:"batch,omitempty"`
	NoPlan          bool     `json:"no_plan,omitempty"`
	NoDedup         bool     `json:"no_dedup,omitempty"`
	NoSummary       bool     `json:"no_summary,omitempty"`
	Preview         bool     `json:"preview,omitempty"`
	TimeoutMinutes  int      `json:"timeout_minutes,omitempty"`
	ResumeSessionID string   `json:"resume_session_id,omitempty"`
}

func (h *ReviewHandler) scanRepository(ctx context.Context, c *app.RequestContext) {
	req, ok := common.Bind[scanRepositoryReq](c)
	if !ok {
		return
	}
	if strings.TrimSpace(req.Provider) == "" || strings.TrimSpace(req.Repo) == "" || strings.TrimSpace(req.HeadSHA) == "" {
		common.Fail(ctx, c, common.BadRequest("provider、repo、head_sha 不能为空"))
		return
	}
	batch := strings.TrimSpace(req.Batch)
	switch batch {
	case "", "none", "by-language", "by-directory":
	default:
		common.Fail(ctx, c, common.BadRequest("batch 仅支持 none、by-language、by-directory"))
		return
	}
	if req.MaxTokensBudget < 0 {
		common.Fail(ctx, c, common.BadRequest("max_tokens_budget 不能为负数"))
		return
	}
	if req.TimeoutMinutes > 0 && (req.TimeoutMinutes < 5 || req.TimeoutMinutes > 120) {
		common.Fail(ctx, c, common.BadRequest("timeout_minutes 范围 5-120"))
		return
	}

	result, err := h.reviewSvc.Scan(ctx, domain.ScanRequest{
		Provider:        strings.TrimSpace(req.Provider),
		Repo:            strings.TrimSpace(req.Repo),
		ProjectID:       strings.TrimSpace(req.ProjectID),
		HeadSHA:         strings.TrimSpace(req.HeadSHA),
		Paths:           cleanStringSlice(req.Paths),
		Exclude:         cleanStringSlice(req.Exclude),
		Background:      strings.TrimSpace(req.Background),
		MaxTokensBudget: req.MaxTokensBudget,
		Batch:           batch,
		NoPlan:          req.NoPlan,
		NoDedup:         req.NoDedup,
		NoSummary:       req.NoSummary,
		Preview:         req.Preview,
		TimeoutMinutes:  req.TimeoutMinutes,
		ResumeSessionID: strings.TrimSpace(req.ResumeSessionID),
	})
	if err != nil {
		common.Fail(ctx, c, mapReviewError(err))
		return
	}

	status := consts.StatusOK
	if !req.Preview && result.ReviewID != "" && result.Result == nil {
		status = consts.StatusAccepted
	}
	c.JSON(status, result)
}

// ScanRepository 包级入口（biz/router 生成代码引用）；一行委托到方法，测试可直接构造 Handler。
func ScanRepository(ctx context.Context, c *app.RequestContext) { handlerInst.scanRepository(ctx, c) }

func cleanStringSlice(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			out = append(out, value)
		}
	}
	return out
}

func (h *ReviewHandler) rerunReview(ctx context.Context, c *app.RequestContext) {
	req, ok := common.Bind[modelReview.ReviewActionReq](c)
	if !ok {
		return
	}

	resp, err := h.reviewSvc.Requeue(ctx, req.ID)
	if err != nil {
		if errors.Is(err, review.ErrNotFound) {
			common.Fail(ctx, c, common.NotFound("review not found"))
			return
		}
		if errors.Is(err, review.ErrInvalidState) {
			common.Fail(ctx, c, common.BizErr(409, err.Error()))
			return
		}
		common.Fail(ctx, c, err)
		return
	}

	converted := convertResponse(resp)
	common.Success(c, &modelReview.CreateReviewResp{Review: &converted})
}

// RerunReview 包级入口（biz/router 生成代码引用）；一行委托到方法，测试可直接构造 Handler。
func RerunReview(ctx context.Context, c *app.RequestContext) { handlerInst.rerunReview(ctx, c) }

func (h *ReviewHandler) cancelReview(ctx context.Context, c *app.RequestContext) {
	req, ok := common.Bind[modelReview.ReviewActionReq](c)
	if !ok {
		return
	}

	resp, err := h.reviewSvc.Cancel(ctx, req.ID)
	if err != nil {
		if errors.Is(err, review.ErrNotFound) {
			common.Fail(ctx, c, common.NotFound("review not found"))
			return
		}
		if errors.Is(err, review.ErrInvalidState) {
			common.Fail(ctx, c, common.BizErr(409, err.Error()))
			return
		}
		common.Fail(ctx, c, err)
		return
	}

	converted := convertResponse(resp)
	common.Success(c, &modelReview.CreateReviewResp{Review: &converted})
}

// CancelReview 包级入口（biz/router 生成代码引用）；一行委托到方法，测试可直接构造 Handler。
func CancelReview(ctx context.Context, c *app.RequestContext) { handlerInst.cancelReview(ctx, c) }

func (h *ReviewHandler) handleWebhook(ctx context.Context, c *app.RequestContext) {
	if webhookInst == nil {
		common.Fail(ctx, c, common.BizErr(500, "webhook handler not initialized"))
		return
	}
	webhookInst.HandleWebhook(ctx, c)
}

// HandleWebhook 包级入口（biz/router 生成代码引用）；一行委托到方法，测试可直接构造 Handler。
func HandleWebhook(ctx context.Context, c *app.RequestContext) { handlerInst.handleWebhook(ctx, c) }

func (h *ReviewHandler) listProviderRepos(ctx context.Context, c *app.RequestContext) {
	req, ok := common.Bind[modelPlatform.ListProviderReposReq](c)
	if !ok {
		return
	}
	if h.platform == nil || !h.platform.HasProvider(req.Provider) {
		common.Fail(ctx, c, common.BadRequest(fmt.Sprintf("未配置的代码平台: %s", req.Provider)))
		return
	}

	pageReq := pagination.NewPageRequest(
		int(ptrx.Deref(req.Page, 1)),
		int(ptrx.Deref(req.PerPage, 30)),
	)
	pageReq.Normalize()
	page, perPage := pageReq.Page, pageReq.PageSize

	repos, err := h.platform.ListRepos(ctx, req.Provider, provider_sdk.ListRepoOptions{Page: page, PerPage: perPage})
	if err != nil {
		common.Fail(ctx, c, common.BizErr(500, fmt.Sprintf("获取仓库列表失败: %v", err)))
		return
	}

	items := slicex.Map(repos, func(r *provider_sdk.PlatformRepo) *modelPlatform.RepoItem {
		return &modelPlatform.RepoItem{
			ID: convert.Int64ToStr(r.ID), FullName: r.FullName, Name: r.Name, Owner: r.Owner,
			Description: common.StrPtr(r.Description), DefaultBranch: common.StrPtr(r.DefaultBranch), Private: r.Private,
		}
	})
	common.Success(c, &modelPlatform.ListProviderReposResp{Items: items})
}

// ListProviderRepos 包级入口（biz/router 生成代码引用）；一行委托到方法，测试可直接构造 Handler。
func ListProviderRepos(ctx context.Context, c *app.RequestContext) {
	handlerInst.listProviderRepos(ctx, c)
}

func (h *ReviewHandler) listProviderBranches(ctx context.Context, c *app.RequestContext) {
	req, ok := common.Bind[modelPlatform.ListProviderBranchesReq](c)
	if !ok {
		return
	}
	if h.platform == nil || !h.platform.HasProvider(req.Provider) {
		common.Fail(ctx, c, common.BadRequest(fmt.Sprintf("未配置的代码平台: %s", req.Provider)))
		return
	}

	branches, err := h.platform.ListBranches(ctx, req.Provider, req.Owner, req.Repo)
	if err != nil {
		common.Fail(ctx, c, common.BizErr(500, fmt.Sprintf("获取分支列表失败: %v", err)))
		return
	}

	items := slicex.Map(branches, func(b *provider_sdk.PlatformBranch) *modelPlatform.BranchItem {
		return &modelPlatform.BranchItem{Name: b.Name}
	})
	common.Success(c, &modelPlatform.ListProviderBranchesResp{Items: items})
}

// ListProviderBranches 包级入口（biz/router 生成代码引用）；一行委托到方法，测试可直接构造 Handler。
func ListProviderBranches(ctx context.Context, c *app.RequestContext) {
	handlerInst.listProviderBranches(ctx, c)
}

func (h *ReviewHandler) listProviderPulls(ctx context.Context, c *app.RequestContext) {
	req, ok := common.Bind[modelPlatform.ListProviderPullsReq](c)
	if !ok {
		return
	}
	if h.platform == nil || !h.platform.HasProvider(req.Provider) {
		common.Fail(ctx, c, common.BadRequest(fmt.Sprintf("未配置的代码平台: %s", req.Provider)))
		return
	}

	pageReq := pagination.NewPageRequest(
		int(ptrx.Deref(req.Page, 1)),
		int(ptrx.Deref(req.PerPage, 30)),
	)
	pageReq.Normalize()
	page, perPage := pageReq.Page, pageReq.PageSize
	state := stringx.TakeOne(ptrx.Deref(req.State, ""), "opened")

	crs, _, err := h.platform.ListCRs(ctx, req.Provider, provider_sdk.ListCROptions{
		Owner: req.Owner, Repo: req.Repo, State: provider_sdk.CRState(state), Page: page, PerPage: perPage,
	})
	if err != nil {
		common.Fail(ctx, c, common.BizErr(500, fmt.Sprintf("获取 PR/MR 列表失败: %v", err)))
		return
	}

	items := slicex.Map(crs, func(cr *provider_sdk.ChangeRequest) *modelPlatform.PullItem {
		item := &modelPlatform.PullItem{
			Number: convert.Str2Int64(cr.Number, 0), Title: cr.Title, State: string(cr.State),
			SourceBranch: cr.SourceBranch, TargetBranch: cr.TargetBranch,
			HeadSha: common.StrPtr(cr.HeadSHA), BaseSha: common.StrPtr(cr.BaseSHA),
			WebURL: common.StrPtr(cr.WebURL), CreatedAt: common.StrPtr(common.FormatTime(cr.CreatedAt)), UpdatedAt: common.StrPtr(common.FormatTime(cr.UpdatedAt)),
		}
		if cr.Author != nil {
			item.Author = &modelPlatform.PRAuthor{
				Username: cr.Author.Username, Name: common.StrPtr(cr.Author.Name), AvatarURL: common.StrPtr(cr.Author.AvatarURL),
			}
		}
		return item
	})
	common.Success(c, &modelPlatform.ListProviderPullsResp{Items: items})
}

// ListProviderPulls 包级入口（biz/router 生成代码引用）；一行委托到方法，测试可直接构造 Handler。
func ListProviderPulls(ctx context.Context, c *app.RequestContext) {
	handlerInst.listProviderPulls(ctx, c)
}

func (h *ReviewHandler) getBranchSHA(ctx context.Context, c *app.RequestContext) {
	req, ok := common.Bind[modelPlatform.GetBranchSHAReq](c)
	if !ok {
		return
	}
	if h.platform == nil || !h.platform.HasProvider(req.Provider) {
		common.Fail(ctx, c, common.BadRequest(fmt.Sprintf("未配置的代码平台: %s", req.Provider)))
		return
	}

	sha, err := h.platform.GetBranchSHA(ctx, req.Provider, req.Owner, req.Repo, req.Branch)
	if err != nil {
		common.Fail(ctx, c, common.BizErr(500, fmt.Sprintf("获取分支 SHA 失败: %v", err)))
		return
	}
	common.Success(c, &modelPlatform.GetBranchSHAResp{Sha: sha})
}

// GetBranchSHA 包级入口（biz/router 生成代码引用）；一行委托到方法，测试可直接构造 Handler。
func GetBranchSHA(ctx context.Context, c *app.RequestContext) { handlerInst.getBranchSHA(ctx, c) }
