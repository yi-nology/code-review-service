package settings_service

import (
	"context"

	"github.com/yi-nology/code-review-service/biz/handler/common"
	modelSettings "github.com/yi-nology/code-review-service/biz/model/settings"
	"github.com/yi-nology/code-review-service/internal/repository/postgres/models"

	"github.com/oekit/pkg/ptrx"
	"github.com/oekit/pkg/slicex"

	"github.com/cloudwego/hertz/pkg/app"
	"gorm.io/datatypes"
)

func ListReviewerAgents(ctx context.Context, c *app.RequestContext) {
	agents, err := handlerInst.agentRepo.List(ctx)
	if err != nil {
		common.Fail(c, err)
		return
	}
	items := slicex.Map(agents, func(a models.ReviewerAgent) *modelSettings.ReviewerAgentInfo {
		return dbReviewerAgentToAPI(&a)
	})
	common.Success(c, &modelSettings.ListReviewerAgentsResp{Agents: items})
}

func CreateReviewerAgent(ctx context.Context, c *app.RequestContext) {
	req, ok := common.Bind[modelSettings.CreateReviewerAgentReq](c)
	if !ok { return }
	agent := &models.ReviewerAgent{
		Name: req.Name, DisplayName: req.DisplayName, Description: ptrx.Deref(req.Description, ""),
		Background: req.Background, Enabled: true,
	}
	if req.Enabled != nil {
		agent.Enabled = *req.Enabled
	}
	if req.Priority != nil {
		agent.Priority = int(*req.Priority)
	}
	if len(req.Tags) > 0 {
		agent.Tags = datatypes.JSON(mustMarshalJSON(req.Tags))
	}
	if err := handlerInst.agentRepo.Create(ctx, agent); err != nil {
		common.Fail(c, err)
		return
	}
	common.Success(c, &modelSettings.CreateReviewerAgentResp{Agent: dbReviewerAgentToAPI(agent)})
}

func UpdateReviewerAgent(ctx context.Context, c *app.RequestContext) {
	req, ok := common.Bind[modelSettings.UpdateReviewerAgentReq](c)
	if !ok { return }
	id, err := parseUintID(req.ID)
	if err != nil {
		common.Fail(c, common.BadRequest("invalid agent id"))
		return
	}
	existing, err := handlerInst.agentRepo.GetByID(ctx, id)
	if err != nil {
		common.Fail(c, common.NotFound("agent not found"))
		return
	}
	if req.DisplayName != nil {
		existing.DisplayName = *req.DisplayName
	}
	if req.Description != nil {
		existing.Description = *req.Description
	}
	if req.Background != nil {
		existing.Background = *req.Background
	}
	if req.Enabled != nil {
		existing.Enabled = *req.Enabled
	}
	if req.Priority != nil {
		existing.Priority = int(*req.Priority)
	}
	if req.Tags != nil {
		existing.Tags = datatypes.JSON(mustMarshalJSON(req.Tags))
	}
	if err := handlerInst.agentRepo.Update(ctx, existing); err != nil {
		common.Fail(c, err)
		return
	}
	common.Success(c, &modelSettings.UpdateReviewerAgentResp{Agent: dbReviewerAgentToAPI(existing)})
}

func DeleteReviewerAgent(ctx context.Context, c *app.RequestContext) {
	req, ok := common.Bind[modelSettings.DeleteReviewerAgentReq](c)
	if !ok { return }
	id, err := parseUintID(req.ID)
	if err != nil {
		common.Fail(c, common.BadRequest("invalid agent id"))
		return
	}
	if err := handlerInst.agentRepo.Delete(ctx, id); err != nil {
		common.Fail(c, err)
		return
	}
	common.Success(c, &modelSettings.DeleteReviewerAgentResp{})
}

func dbReviewerAgentToAPI(a *models.ReviewerAgent) *modelSettings.ReviewerAgentInfo {
	if a == nil {
		return nil
	}
	var tags string
	if len(a.Tags) > 0 {
		tags = string(a.Tags)
	}
	return &modelSettings.ReviewerAgentInfo{
		ID: safeInt64(a.ID), Name: a.Name, DisplayName: a.DisplayName,
		Description: common.StrPtr(a.Description), Background: common.StrPtr(a.Background),
		Enabled: a.Enabled, Priority: int64(a.Priority), Tags: common.StrPtr(tags),
	}
}
