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
	"github.com/yi-nology/code-review-service/internal/service/provider"
	"github.com/yi-nology/code-review-service/internal/service/review"
	"github.com/yi-nology/code-review-service/internal/service/review/domain"

	"github.com/oekit/pkg/convert"
	"github.com/oekit/pkg/mathx"
	"github.com/oekit/pkg/ptrx"
	"github.com/oekit/pkg/slicex"
	"github.com/oekit/pkg/stringx"

	"github.com/oekit/logx"
	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	provider_sdk "github.com/yi-nology/git-platform-sdk/provider"
)

var handlerInst *ReviewHandler
var webhookInst *WebhookHandler

type ReviewHandler struct {
	reviewSvc svcApp.ReviewService
	gitMgr    svcApp.GitProviderManager
}

func InitHandler(svc svcApp.ReviewService, gitMgr svcApp.GitProviderManager) {
	handlerInst = &ReviewHandler{
		reviewSvc: svc,
		gitMgr:    gitMgr,
	}
}

// ==================== Review ====================

func CreateReview(ctx context.Context, c *app.RequestContext) {
	req, ok := common.Bind[modelReview.CreateReviewReq](c)
	if !ok {
		return
	}

	svcReq := toServiceRequest(req)
	fillAuthorInfo(ctx, &svcReq)

	idempotencyKey := string(c.GetHeader("Idempotency-Key"))
	resp, err := handlerInst.reviewSvc.Create(ctx, svcReq, idempotencyKey)
	if err != nil {
		common.Fail(c, mapReviewError(err))
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

func GetReview(ctx context.Context, c *app.RequestContext) {
	req, ok := common.Bind[modelReview.GetReviewReq](c)
	if !ok {
		return
	}

	record, ok, err := handlerInst.reviewSvc.Get(ctx, req.ID)
	if err != nil {
		common.Fail(c, err)
		return
	}
	if !ok {
		common.Fail(c, common.NotFound("review not found"))
		return
	}

	detail := convertDetail(record)
	common.Success(c, &modelReview.GetReviewResp{Review: &detail})
}

func ListReviews(ctx context.Context, c *app.RequestContext) {
	req, ok := common.Bind[modelReview.ListReviewsReq](c)
	if !ok {
		return
	}

	page, pageSize := common.PageParams(
		convert.Int64ToStr(ptrx.Deref(req.Page, 0)),
		convert.Int64ToStr(ptrx.Deref(req.PageSize, 0)),
	)

	result, err := handlerInst.reviewSvc.List(ctx, review.ListOptions{
		Status:   strings.TrimSpace(ptrx.Deref(req.Status, "")),
		Provider: strings.TrimSpace(ptrx.Deref(req.Provider, "")),
		Repo:     strings.TrimSpace(ptrx.Deref(req.Repo, "")),
		Limit:    pageSize,
		Offset:   (page - 1) * pageSize,
	})
	if err != nil {
		common.Fail(c, err)
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

func PreviewReview(ctx context.Context, c *app.RequestContext) {
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

	files, err := handlerInst.reviewSvc.Preview(ctx, svcReq)
	if err != nil {
		common.Fail(c, mapReviewError(err))
		return
	}

	common.Success(c, &modelReview.PreviewResp{Files: files})
}

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
}

func ScanRepository(ctx context.Context, c *app.RequestContext) {
	req, ok := common.Bind[scanRepositoryReq](c)
	if !ok {
		return
	}
	if strings.TrimSpace(req.Provider) == "" || strings.TrimSpace(req.Repo) == "" || strings.TrimSpace(req.HeadSHA) == "" {
		common.Fail(c, common.BadRequest("provider、repo、head_sha 不能为空"))
		return
	}
	batch := strings.TrimSpace(req.Batch)
	switch batch {
	case "", "none", "by-language", "by-directory":
	default:
		common.Fail(c, common.BadRequest("batch 仅支持 none、by-language、by-directory"))
		return
	}
	if req.MaxTokensBudget < 0 {
		common.Fail(c, common.BadRequest("max_tokens_budget 不能为负数"))
		return
	}

	result, err := handlerInst.reviewSvc.Scan(ctx, domain.ScanRequest{
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
	})
	if err != nil {
		common.Fail(c, mapReviewError(err))
		return
	}

	status := consts.StatusOK
	if !req.Preview && result.ReviewID != "" && result.Result == nil {
		status = consts.StatusAccepted
	}
	c.JSON(status, result)
}

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

func RerunReview(ctx context.Context, c *app.RequestContext) {
	req, ok := common.Bind[modelReview.ReviewActionReq](c)
	if !ok {
		return
	}

	resp, err := handlerInst.reviewSvc.Requeue(ctx, req.ID)
	if err != nil {
		if errors.Is(err, review.ErrNotFound) {
			common.Fail(c, common.NotFound("review not found"))
			return
		}
		if errors.Is(err, review.ErrInvalidState) {
			common.Fail(c, common.BizErr(409, err.Error()))
			return
		}
		common.Fail(c, err)
		return
	}

	converted := convertResponse(resp)
	common.Success(c, &modelReview.CreateReviewResp{Review: &converted})
}

func CancelReview(ctx context.Context, c *app.RequestContext) {
	req, ok := common.Bind[modelReview.ReviewActionReq](c)
	if !ok {
		return
	}

	resp, err := handlerInst.reviewSvc.Cancel(ctx, req.ID)
	if err != nil {
		if errors.Is(err, review.ErrNotFound) {
			common.Fail(c, common.NotFound("review not found"))
			return
		}
		if errors.Is(err, review.ErrInvalidState) {
			common.Fail(c, common.BizErr(409, err.Error()))
			return
		}
		common.Fail(c, err)
		return
	}

	converted := convertResponse(resp)
	common.Success(c, &modelReview.CreateReviewResp{Review: &converted})
}

// ==================== Webhook ====================

func HandleWebhook(ctx context.Context, c *app.RequestContext) {
	if webhookInst == nil {
		common.Fail(c, common.BizErr(500, "webhook handler not initialized"))
		return
	}
	webhookInst.HandleWebhook(ctx, c)
}

// ==================== Platform ====================

func ListProviderRepos(ctx context.Context, c *app.RequestContext) {
	req, ok := common.Bind[modelPlatform.ListProviderReposReq](c)
	if !ok {
		return
	}
	if handlerInst.gitMgr == nil || !handlerInst.gitMgr.HasProvider(req.Provider) {
		common.Fail(c, common.BadRequest(fmt.Sprintf("未配置的代码平台: %s", req.Provider)))
		return
	}

	page := mathx.Clamp(int(ptrx.Deref(req.Page, 1)), 1, 999999)
	perPage := mathx.Clamp(int(ptrx.Deref(req.PerPage, 30)), 1, 100)

	repos, err := handlerInst.gitMgr.ListRepos(ctx, req.Provider, provider_sdk.ListRepoOptions{Page: page, PerPage: perPage})
	if err != nil {
		common.Fail(c, common.BizErr(500, fmt.Sprintf("获取仓库列表失败: %v", err)))
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

func ListProviderBranches(ctx context.Context, c *app.RequestContext) {
	req, ok := common.Bind[modelPlatform.ListProviderBranchesReq](c)
	if !ok {
		return
	}
	if handlerInst.gitMgr == nil || !handlerInst.gitMgr.HasProvider(req.Provider) {
		common.Fail(c, common.BadRequest(fmt.Sprintf("未配置的代码平台: %s", req.Provider)))
		return
	}

	branches, err := handlerInst.gitMgr.ListBranches(ctx, req.Provider, req.Owner, req.Repo)
	if err != nil {
		common.Fail(c, common.BizErr(500, fmt.Sprintf("获取分支列表失败: %v", err)))
		return
	}

	items := slicex.Map(branches, func(b *provider_sdk.PlatformBranch) *modelPlatform.BranchItem {
		return &modelPlatform.BranchItem{Name: b.Name}
	})
	common.Success(c, &modelPlatform.ListProviderBranchesResp{Items: items})
}

func ListProviderPulls(ctx context.Context, c *app.RequestContext) {
	req, ok := common.Bind[modelPlatform.ListProviderPullsReq](c)
	if !ok {
		return
	}
	if handlerInst.gitMgr == nil || !handlerInst.gitMgr.HasProvider(req.Provider) {
		common.Fail(c, common.BadRequest(fmt.Sprintf("未配置的代码平台: %s", req.Provider)))
		return
	}

	page := mathx.Clamp(int(ptrx.Deref(req.Page, 1)), 1, 999999)
	perPage := mathx.Clamp(int(ptrx.Deref(req.PerPage, 30)), 1, 100)
	state := stringx.TakeOne(ptrx.Deref(req.State, ""), "opened")

	crs, _, err := handlerInst.gitMgr.ListCRs(ctx, req.Provider, provider_sdk.ListCROptions{
		Owner: req.Owner, Repo: req.Repo, State: provider_sdk.CRState(state), Page: page, PerPage: perPage,
	})
	if err != nil {
		common.Fail(c, common.BizErr(500, fmt.Sprintf("获取 PR/MR 列表失败: %v", err)))
		return
	}

	items := slicex.Map(crs, func(cr *provider_sdk.ChangeRequest) *modelPlatform.PullItem {
		item := &modelPlatform.PullItem{
			Number: int64(cr.Number), Title: cr.Title, State: string(cr.State),
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

func GetBranchSHA(ctx context.Context, c *app.RequestContext) {
	req, ok := common.Bind[modelPlatform.GetBranchSHAReq](c)
	if !ok {
		return
	}
	if handlerInst.gitMgr == nil || !handlerInst.gitMgr.HasProvider(req.Provider) {
		common.Fail(c, common.BadRequest(fmt.Sprintf("未配置的代码平台: %s", req.Provider)))
		return
	}

	sha, err := handlerInst.gitMgr.GetBranchSHA(ctx, req.Provider, req.Owner, req.Repo, req.Branch)
	if err != nil {
		common.Fail(c, common.BizErr(500, fmt.Sprintf("获取分支 SHA 失败: %v", err)))
		return
	}
	common.Success(c, &modelPlatform.GetBranchSHAResp{Sha: sha})
}

// ==================== Converters ====================

func mapReviewError(err error) error {
	switch {
	case errors.Is(err, domain.ErrUnsupportedProvider):
		return common.BizErr(422, err.Error())
	case errors.Is(err, domain.ErrIdempotencyConflict):
		return common.BizErr(409, err.Error())
	default:
		return err
	}
}

func fillAuthorInfo(ctx context.Context, req *domain.Request) {
	if req.Author != "" {
		return
	}
	prID := stringx.TakeOne(req.PullRequestID, req.MergeRequestIID)
	if prID == "" {
		return
	}
	number := convert.Str2Int(prID, 0)
	if number <= 0 || handlerInst.gitMgr == nil || !handlerInst.gitMgr.HasProvider(req.Provider) {
		return
	}
	owner, repo, err := provider.ParseRepoPath(req.Repo)
	if err != nil {
		return
	}
	cr, err := handlerInst.gitMgr.GetCR(ctx, req.Provider, owner, repo, number)
	if err != nil {
		logx.CtxWarnf(ctx, "fillAuthorInfo: failed to get CR for %s/%s #%d: %v", owner, repo, number, err)
		return
	}
	if cr == nil || cr.Author == nil {
		return
	}
	req.Author = cr.Author.Username
	req.AuthorName = stringx.TakeOne(cr.Author.Name, cr.Author.Username)
	req.AuthorAvatar = cr.Author.AvatarURL
}

func toServiceRequest(req *modelReview.CreateReviewReq) domain.Request {
	svcReq := domain.Request{
		Provider:        req.Provider,
		Repo:            req.Repo,
		ProjectID:       ptrx.Deref(req.ProjectID, ""),
		PullRequestID:   ptrx.Deref(req.PullRequestID, ""),
		MergeRequestIID: ptrx.Deref(req.MergeRequestIid, ""),
		BaseSHA:         ptrx.Deref(req.BaseSha, ""),
		HeadSHA:         ptrx.Deref(req.HeadSha, ""),
		CallbackMode:    ptrx.Deref(req.CallbackMode, ""),
		Wait:            ptrx.Deref(req.Wait, false),
		Requirements:    req.Requirements,
	}
	if len(req.EphemeralReviewers) > 0 {
		svcReq.EphemeralReviewers = make([]domain.EphemeralReviewer, len(req.EphemeralReviewers))
		for i, e := range req.EphemeralReviewers {
			svcReq.EphemeralReviewers[i] = domain.EphemeralReviewer{Name: ptrx.Deref(e.Name, ""), Background: e.Background}
		}
	}
	return svcReq
}

func convertGate(g domain.Gate) *modelReview.Gate {
	return &modelReview.Gate{Status: g.Status, Passed: g.Passed, Reason: common.StrPtr(g.Reason)}
}

func convertResponse(r domain.Response) modelReview.ReviewResponse {
	return modelReview.ReviewResponse{
		ReviewID: r.ReviewID, Status: r.Status, Summary: r.Summary,
		Gate: convertGate(r.Gate), Report: convertReport(r.Report),
		Analysis: convertAnalysis(r.Analysis), History: convertEvents(r.History),
		Findings: convertFindings(r.Findings), Error: common.StrPtr(r.Error),
	}
}

func convertDetail(r domain.Record) modelReview.ReviewDetail {
	return modelReview.ReviewDetail{
		ReviewID: r.ID, Provider: r.Request.Provider, Repo: r.Request.Repo,
		ProjectID: common.StrPtr(r.Request.ProjectID), PullRequestID: common.StrPtr(r.Request.PullRequestID),
		MergeRequestIid: common.StrPtr(r.Request.MergeRequestIID), BaseSha: common.StrPtr(r.Request.BaseSHA),
		HeadSha: common.StrPtr(r.Request.HeadSHA), CallbackMode: common.StrPtr(r.Request.CallbackMode),
		Status: r.Response.Status, Summary: r.Response.Summary,
		Gate: convertGate(r.Response.Gate), Analysis: convertAnalysis(r.Response.Analysis),
		History: convertEvents(r.Response.History), Findings: convertFindings(r.Response.Findings),
		Report: convertReport(r.Response.Report), RawOcr: common.StrPtr(string(r.RawOCR)),

		Error: common.StrPtr(r.Response.Error), Author: common.StrPtr(r.Request.Author),
		AuthorName: common.StrPtr(r.Request.AuthorName), AuthorAvatar: common.StrPtr(r.Request.AuthorAvatar),
		CreatedAt: common.StrPtr(common.FormatTime(r.CreatedAt)),
		UpdatedAt: common.StrPtr(common.FormatTime(r.UpdatedAt)),
	}
}

func convertReport(r domain.Report) *modelReview.Report {
	return &modelReview.Report{
		Ocr: convertOCRMetadata(r.OCR), Scope: convertScopeMetadata(r.Scope),
		Score: &modelReview.Score{
			Value: int64(r.Score.Value), Grade: r.Score.Grade, Version: r.Score.Version,
			Deductions:   convertDeductions(r.Score.Deductions),
			ChangedLines: ptrx.Of(int64(r.Score.ChangedLines)), FindingsDensity: ptrx.Of(r.Score.FindingsDensity),
		},
		Counts: &modelReview.SeverityCounts{
			Critical: int64(r.Counts.Critical), Major: int64(r.Counts.Major),
			Minor: int64(r.Counts.Minor), Info: int64(r.Counts.Info),
		},
		Generated: ptrx.Of(r.Generated), Complete: ptrx.Of(r.Complete),
	}
}

func convertOCRMetadata(o domain.OCRMetadata) *modelReview.OCRMetadata {
	item := &modelReview.OCRMetadata{
		Engine: common.StrPtr(o.Engine), Status: common.StrPtr(o.Status),
		FilesReviewed: ptrx.Of(o.FilesReviewed), Comments: ptrx.Of(o.Comments),
		TotalTokens: ptrx.Of(o.TotalTokens), InputTokens: ptrx.Of(o.InputTokens),
		OutputTokens: ptrx.Of(o.OutputTokens), CacheReadTokens: ptrx.Of(o.CacheReadTokens),
		CacheWriteTokens: ptrx.Of(o.CacheWriteTokens), Elapsed: common.StrPtr(o.Elapsed),
	}
	if o.ToolCalls != nil {
		item.ToolCalls = &modelReview.OCRToolCalls{
			Total:  o.ToolCalls.Total,
			ByTool: o.ToolCalls.ByTool,
		}
	}
	if strings.TrimSpace(o.ProjectSummary) != "" {
		item.ProjectSummary = common.StrPtr(o.ProjectSummary)
	}
	return item
}

func convertDeductions(ds []domain.ScoreDeduction) []*modelReview.ScoreDeduction {
	if len(ds) == 0 {
		return nil
	}
	return slicex.Map(ds, func(d domain.ScoreDeduction) *modelReview.ScoreDeduction {
		return &modelReview.ScoreDeduction{Reason: d.Reason, Points: int64(d.Points), Count: ptrx.Of(int64(d.Count))}
	})
}

func convertScopeMetadata(s domain.ScopeMetadata) *modelReview.ScopeMetadata {
	return &modelReview.ScopeMetadata{
		ChangedFiles: int64(s.ChangedFiles), ReviewedFiles: int64(s.ReviewedFiles),
		ExcludedFiles: int64(s.ExcludedFiles), IgnorePaths: s.IgnorePaths,
	}
}

func convertAnalysis(a domain.Analysis) *modelReview.Analysis {
	return &modelReview.Analysis{
		Overview: common.StrPtr(a.Overview), RootCause: a.RootCause,
		Risks: a.Risks, Recommendations: a.Recommendations,
	}
}

func convertEvents(events []domain.Event) []*modelReview.Event {
	if len(events) == 0 {
		return nil
	}
	return slicex.Map(events, func(e domain.Event) *modelReview.Event {
		details := make(map[string]string, len(e.Details))
		for k, v := range e.Details {
			details[k] = convert.AnyToStr(v)
		}
		return &modelReview.Event{
			Stage: e.Stage, Status: e.Status, Message: e.Message, Details: details,
			CreatedAt: common.StrPtr(common.FormatTime(e.CreatedAt)),
		}
	})
}

func convertFindings(findings []domain.Finding) []*modelReview.Finding {
	if len(findings) == 0 {
		return nil
	}
	return slicex.Map(findings, func(f domain.Finding) *modelReview.Finding {
		return &modelReview.Finding{
			Severity: f.Severity, Category: common.StrPtr(f.Category), Path: common.StrPtr(f.Path),
			Line: ptrx.Of(int64(f.Line)), StartLine: ptrx.Of(int64(f.StartLine)), EndLine: ptrx.Of(int64(f.EndLine)),
			Title: f.Title, Body: f.Body, ExistingCode: common.StrPtr(f.ExistingCode),
			SuggestionCode: common.StrPtr(f.SuggestionCode),
		}
	})
}
