package review_service

import (
	"context"
	"errors"
	"fmt"

	modelReview "github.com/yi-nology/code-review-service/biz/model/review"
	"github.com/yi-nology/code-review-service/internal/service/review"

	hertzApp "github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
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
		ProjectID:       derefStr(req.ProjectID),
		PullRequestID:   derefStr(req.PullRequestID),
		MergeRequestIID: derefStr(req.MergeRequestIid),
		BaseSHA:         derefStr(req.BaseSha),
		HeadSHA:         derefStr(req.HeadSha),
		CallbackMode:    derefStr(req.CallbackMode),
		Wait:            derefBool(req.Wait),
	}

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
		fmt.Sprintf("%d", derefInt64(req.Page)),
		fmt.Sprintf("%d", derefInt64(req.PageSize)),
	)

	result, err := h.reviewSvc.List(ctx, review.ListOptions{
		Status:   cleanQuery(derefStr(req.Status)),
		Provider: cleanQuery(derefStr(req.Provider)),
		Repo:     cleanQuery(derefStr(req.Repo)),
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
			Author:          item.Author,
			AuthorName:      item.AuthorName,
			AuthorAvatar:    item.AuthorAvatar,
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
		ProjectID:       derefStr(req.ProjectID),
		PullRequestID:   derefStr(req.PullRequestID),
		MergeRequestIID: derefStr(req.MergeRequestIID),
		BaseSHA:         derefStr(req.BaseSHA),
		HeadSHA:         derefStr(req.HeadSHA),
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
		if errors.Is(err, review.ErrNotFound) {
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
		if errors.Is(err, review.ErrNotFound) {
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
