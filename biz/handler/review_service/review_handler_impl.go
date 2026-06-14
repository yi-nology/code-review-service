package review_service

import (
	"context"
	"errors"
	"strings"

	modelReview "github.com/yi-nology/code-review-service/biz/model/review"
	"github.com/yi-nology/code-review-service/internal/service/provider"
	"github.com/yi-nology/code-review-service/internal/service/review"

	"github.com/oekit/pkg/convert"
	"github.com/oekit/pkg/ptrx"
	"github.com/oekit/pkg/stringx"

	hertzApp "github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/hlog"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	gormLib "gorm.io/gorm"
)

func mapReviewError(err error) error {
	switch {
	case errors.Is(err, review.ErrUnsupportedProvider):
		return bizErr(422, err.Error())
	case errors.Is(err, review.ErrIdempotencyConflict):
		return bizErr(409, err.Error())
	default:
		return err
	}
}

func (h *ReviewHandler) CreateReview(ctx context.Context, c *hertzApp.RequestContext) {
	var req modelReview.CreateReviewReq
	if err := c.BindAndValidate(&req); err != nil {
		fail(c, badRequest(err.Error()))
		return
	}

	svcReq := review.Request{
		Provider:        req.Provider,
		Repo:            req.Repo,
		ProjectID:       ptrx.Deref(req.ProjectID, ""),
		PullRequestID:   ptrx.Deref(req.PullRequestID, ""),
		MergeRequestIID: ptrx.Deref(req.MergeRequestIid, ""),
		BaseSHA:         ptrx.Deref(req.BaseSha, ""),
		HeadSHA:         ptrx.Deref(req.HeadSha, ""),
		CallbackMode:    ptrx.Deref(req.CallbackMode, ""),
		Wait:            ptrx.Deref(req.Wait, false),
	}

	h.fillAuthorInfo(ctx, &svcReq)

	idempotencyKey := string(c.GetHeader("Idempotency-Key"))
	resp, err := h.reviewSvc.Create(ctx, svcReq, idempotencyKey)
	if err != nil {
		fail(c, mapReviewError(err))
		return
	}

	converted := convertResponse(resp)
	apiResp := &modelReview.CreateReviewResp{
		Review: &converted,
	}

	status := consts.StatusOK
	if resp.Status == review.StatusQueued {
		status = consts.StatusAccepted
	}
	c.JSON(status, apiResp)
}

func (h *ReviewHandler) GetReview(ctx context.Context, c *hertzApp.RequestContext) {
	var req modelReview.GetReviewReq
	if err := c.BindAndValidate(&req); err != nil {
		fail(c, badRequest(err.Error()))
		return
	}

	record, ok, err := h.reviewSvc.Get(ctx, req.ID)
	if err != nil {
		fail(c, err)
		return
	}
	if !ok {
		fail(c, notFound("review not found"))
		return
	}

	detail := convertDetail(record)
	c.JSON(consts.StatusOK, &modelReview.GetReviewResp{
		Review: &detail,
	})
}

func (h *ReviewHandler) ListReviews(ctx context.Context, c *hertzApp.RequestContext) {
	var req modelReview.ListReviewsReq
	if err := c.BindAndValidate(&req); err != nil {
		fail(c, badRequest(err.Error()))
		return
	}

	page, pageSize := pageParams(
		convert.Int64ToStr(ptrx.Deref(req.Page, 0)),
		convert.Int64ToStr(ptrx.Deref(req.PageSize, 0)),
	)

	result, err := h.reviewSvc.List(ctx, review.ListOptions{
		Status:   cleanQuery(ptrx.Deref(req.Status, "")),
		Provider: cleanQuery(ptrx.Deref(req.Provider, "")),
		Repo:     cleanQuery(ptrx.Deref(req.Repo, "")),
		Limit:    pageSize,
		Offset:   (page - 1) * pageSize,
	})
	if err != nil {
		fail(c, err)
		return
	}

	items := make([]*modelReview.ListItem, 0, len(result.Items))
	for _, item := range result.Items {
		items = append(items, &modelReview.ListItem{
			ReviewID:        item.ReviewID,
			Provider:        item.Provider,
			Repo:            item.Repo,
			Status:          item.Status,
			Summary:         item.Summary,
			Gate:            convertGate(item.Gate),
			Findings:        int64(item.Findings),
			Score:           int64(item.Score),
			Grade:           strPtr(item.Grade),
			ReportGenerated: item.ReportGenerated,
			Author:          strPtr(item.Author),
			AuthorName:      strPtr(item.AuthorName),
			AuthorAvatar:    strPtr(item.AuthorAvatar),
			CreatedAt:       strPtr(item.CreatedAt.Format(timeFormat)),
			UpdatedAt:       strPtr(item.UpdatedAt.Format(timeFormat)),
		})
	}

	c.JSON(consts.StatusOK, &modelReview.ListReviewsResp{
		Items:      items,
		Total:      int64(result.Total),
		Page:       int64(result.Page),
		PageSize:   int64(result.PageSize),
		TotalPages: int64(result.TotalPages),
		Stats: &modelReview.ListStats{
			Total:      int64(result.Stats.Total),
			Running:    int64(result.Stats.Running),
			GateFailed: int64(result.Stats.GateFailed),
			Findings:   int64(result.Stats.Findings),
		},
	})
}

func (h *ReviewHandler) PreviewReview(ctx context.Context, c *hertzApp.RequestContext) {
	var req struct {
		Provider        string  `json:"provider" form:"provider"`
		Repo            string  `json:"repo" form:"repo"`
		ProjectID       *string `json:"project_id,omitempty" form:"project_id"`
		PullRequestID   *string `json:"pull_request_id,omitempty" form:"pull_request_id"`
		MergeRequestIID *string `json:"merge_request_iid,omitempty" form:"merge_request_iid"`
		BaseSHA         *string `json:"base_sha,omitempty" form:"base_sha"`
		HeadSHA         *string `json:"head_sha,omitempty" form:"head_sha"`
	}
	if err := c.BindAndValidate(&req); err != nil {
		fail(c, badRequest(err.Error()))
		return
	}

	svcReq := review.Request{
		Provider:        req.Provider,
		Repo:            req.Repo,
		ProjectID:       ptrx.Deref(req.ProjectID, ""),
		PullRequestID:   ptrx.Deref(req.PullRequestID, ""),
		MergeRequestIID: ptrx.Deref(req.MergeRequestIID, ""),
		BaseSHA:         ptrx.Deref(req.BaseSHA, ""),
		HeadSHA:         ptrx.Deref(req.HeadSHA, ""),
	}

	files, err := h.reviewSvc.Preview(ctx, svcReq)
	if err != nil {
		fail(c, mapReviewError(err))
		return
	}

	c.JSON(consts.StatusOK, map[string]any{
		"files": files,
	})
}

func (h *ReviewHandler) RerunReview(ctx context.Context, c *hertzApp.RequestContext) {
	reviewID := c.Param("id")
	if reviewID == "" {
		fail(c, badRequest("review id is required"))
		return
	}

	resp, err := h.reviewSvc.Requeue(ctx, reviewID)
	if err != nil {
		if errors.Is(err, review.ErrNotFound) || errors.Is(err, gormLib.ErrRecordNotFound) || strings.Contains(err.Error(), "not found") {
			fail(c, notFound("review not found"))
			return
		}
		if errors.Is(err, review.ErrInvalidState) {
			fail(c, bizErr(409, err.Error()))
			return
		}
		fail(c, err)
		return
	}

	converted := convertResponse(resp)
	c.JSON(consts.StatusOK, &modelReview.CreateReviewResp{
		Review: &converted,
	})
}

func (h *ReviewHandler) CancelReview(ctx context.Context, c *hertzApp.RequestContext) {
	reviewID := c.Param("id")
	if reviewID == "" {
		fail(c, badRequest("review id is required"))
		return
	}

	resp, err := h.reviewSvc.Cancel(ctx, reviewID)
	if err != nil {
		if errors.Is(err, review.ErrNotFound) || errors.Is(err, gormLib.ErrRecordNotFound) || strings.Contains(err.Error(), "not found") {
			fail(c, notFound("review not found"))
			return
		}
		if errors.Is(err, review.ErrInvalidState) {
			fail(c, bizErr(409, err.Error()))
			return
		}
		fail(c, err)
		return
	}

	converted := convertResponse(resp)
	c.JSON(consts.StatusOK, &modelReview.CreateReviewResp{
		Review: &converted,
	})
}

func (h *ReviewHandler) fillAuthorInfo(ctx context.Context, req *review.Request) {
	if req.Author != "" {
		return
	}
	prID := stringx.TakeOne(req.PullRequestID, req.MergeRequestIID)
	if prID == "" {
		return
	}
	number := convert.Str2Int(prID, 0)
	if number <= 0 {
		return
	}
	if h.gitMgr == nil || !h.gitMgr.HasProvider(req.Provider) {
		return
	}
	owner, repo, err := provider.ParseRepoPath(req.Repo)
	if err != nil {
		return
	}
	cr, err := h.gitMgr.GetCR(ctx, req.Provider, owner, repo, number)
	if err != nil {
		hlog.CtxWarnf(ctx, "fillAuthorInfo: failed to get CR for %s/%s #%d: %v", owner, repo, number, err)
		return
	}
	if cr == nil || cr.Author == nil {
		return
	}
	req.Author = cr.Author.Username
	req.AuthorName = stringx.TakeOne(cr.Author.Name, cr.Author.Username)
	req.AuthorAvatar = cr.Author.AvatarURL
}
