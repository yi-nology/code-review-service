package feedback_service

import (
	"context"
	"strings"
	"time"

	"github.com/yi-nology/code-review-service/biz/handler/common"
	"github.com/yi-nology/code-review-service/internal/domain/feedback"
	"github.com/yi-nology/code-review-service/internal/repository"

	"github.com/cloudwego/hertz/pkg/app"
)

var handlerInst *Handler

type Handler struct {
	repo repository.FindingFeedbackRepository
}

func InitHandler(repo repository.FindingFeedbackRepository) {
	handlerInst = &Handler{repo: repo}
}

type upsertFeedbackReq struct {
	Status      string `json:"status"`
	Note        string `json:"note"`
	Fingerprint string `json:"fingerprint"`
}

func (h *Handler) listFeedback(ctx context.Context, c *app.RequestContext) {
	if h == nil || h.repo == nil {
		common.Fail(ctx, c, common.BizErr(500, "feedback handler not initialized"))
		return
	}
	reviewID := c.Param("id")
	items, err := h.repo.ListByReview(ctx, reviewID)
	if err != nil {
		common.Fail(ctx, c, err)
		return
	}
	common.Success(c, map[string]any{"items": items})
}

// ListFeedback 包级入口（biz/router 生成代码引用）；一行委托到方法，便于测试直接构造 Handler。
func ListFeedback(ctx context.Context, c *app.RequestContext) { handlerInst.listFeedback(ctx, c) }

func (h *Handler) upsertFeedback(ctx context.Context, c *app.RequestContext) {
	if h == nil || h.repo == nil {
		common.Fail(ctx, c, common.BizErr(500, "feedback handler not initialized"))
		return
	}
	req, ok := common.Bind[upsertFeedbackReq](c)
	if !ok {
		return
	}
	status := strings.TrimSpace(req.Status)
	switch status {
	case "false_positive", "accepted_risk", "deferred", "fixed":
	default:
		common.Fail(ctx, c, common.BadRequest("status 仅支持 false_positive、accepted_risk、deferred、fixed"))
		return
	}
	index, err := common.ParseUintID(c.Param("index"))
	if err != nil {
		common.Fail(ctx, c, common.BadRequest("invalid finding index"))
		return
	}
	item := &feedback.Feedback{
		ReviewID:     c.Param("id"),
		FindingIndex: int(index),
		Fingerprint:  strings.TrimSpace(req.Fingerprint),
		Status:       status,
		Note:         strings.TrimSpace(req.Note),
		UpdatedAt:    time.Now().UTC(),
	}
	if err := h.repo.Upsert(ctx, item); err != nil {
		common.Fail(ctx, c, err)
		return
	}
	common.Success(c, item)
}

// UpsertFeedback 包级入口（biz/router 生成代码引用）；一行委托到方法，便于测试直接构造 Handler。
func UpsertFeedback(ctx context.Context, c *app.RequestContext) { handlerInst.upsertFeedback(ctx, c) }

type lookupFeedbackReq struct {
	Fingerprints []string `json:"fingerprints"`
}

// ListFeedbackByFingerprints 按指纹集合批量查询历史反馈，用于跨 review 误报抑制。
// 前端可据此标注"该问题曾被标记为误报"，评审引擎也可据此自动降级。
// POST /api/v1/findings/feedback/lookup  body: {"fingerprints": ["abc","def"]}
func (h *Handler) listFeedbackByFingerprints(ctx context.Context, c *app.RequestContext) {
	if h == nil || h.repo == nil {
		common.Fail(ctx, c, common.BizErr(500, "feedback handler not initialized"))
		return
	}
	req, ok := common.Bind[lookupFeedbackReq](c)
	if !ok {
		return
	}
	items, err := h.repo.ListByFingerprints(ctx, req.Fingerprints)
	if err != nil {
		common.Fail(ctx, c, err)
		return
	}
	common.Success(c, map[string]any{"items": items})
}

// ListFeedbackByFingerprints 包级入口（biz/router 生成代码引用）；一行委托到方法，便于测试直接构造 Handler。
func ListFeedbackByFingerprints(ctx context.Context, c *app.RequestContext) {
	handlerInst.listFeedbackByFingerprints(ctx, c)
}
