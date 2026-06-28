package feedback_service

import (
	"context"
	"strings"
	"time"

	"github.com/yi-nology/code-review-service/biz/handler/common"
	"github.com/yi-nology/code-review-service/internal/repository"
	"github.com/yi-nology/code-review-service/internal/repository/postgres/models"

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
	Status string `json:"status"`
	Note   string `json:"note"`
}

func ListFeedback(ctx context.Context, c *app.RequestContext) {
	if handlerInst == nil || handlerInst.repo == nil {
		common.Fail(c, common.BizErr(500, "feedback handler not initialized"))
		return
	}
	reviewID := c.Param("id")
	items, err := handlerInst.repo.ListByReview(ctx, reviewID)
	if err != nil {
		common.Fail(c, err)
		return
	}
	common.Success(c, map[string]any{"items": items})
}

func UpsertFeedback(ctx context.Context, c *app.RequestContext) {
	if handlerInst == nil || handlerInst.repo == nil {
		common.Fail(c, common.BizErr(500, "feedback handler not initialized"))
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
		common.Fail(c, common.BadRequest("status 仅支持 false_positive、accepted_risk、deferred、fixed"))
		return
	}
	index, err := common.ParseUintID(c.Param("index"))
	if err != nil {
		common.Fail(c, common.BadRequest("invalid finding index"))
		return
	}
	item := &models.FindingFeedback{
		ReviewID:     c.Param("id"),
		FindingIndex: int(index),
		Status:       status,
		Note:         strings.TrimSpace(req.Note),
		UpdatedAt:    time.Now().UTC(),
	}
	if err := handlerInst.repo.Upsert(ctx, item); err != nil {
		common.Fail(c, err)
		return
	}
	common.Success(c, item)
}
