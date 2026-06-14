package project_agent_service

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/yi-nology/code-review-service/biz/handler/common"
	modelProjectAgent "github.com/yi-nology/code-review-service/biz/model/project_agent"
	"github.com/yi-nology/code-review-service/internal/repository"
	"github.com/yi-nology/code-review-service/internal/repository/postgres/models"
	"github.com/oekit/pkg/ptrx"
	"github.com/oekit/pkg/slicex"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/hlog"
)

var handlerInst *ProjectAgentHandler

type ProjectAgentHandler struct {
	projectAgentRepo  repository.ProjectAgentRepository
	reviewerAgentRepo repository.ReviewerAgentRepository
}

func InitHandler(
	projectAgentRepo repository.ProjectAgentRepository,
	reviewerAgentRepo repository.ReviewerAgentRepository,
) {
	handlerInst = &ProjectAgentHandler{
		projectAgentRepo:  projectAgentRepo,
		reviewerAgentRepo: reviewerAgentRepo,
	}
}

func GetProjectAgents(ctx context.Context, c *app.RequestContext) {
	var req modelProjectAgent.GetProjectAgentsReq
	if err := c.BindAndValidate(&req); err != nil {
		common.Fail(c, common.BadRequest(err.Error()))
		return
	}

	projectAgents, err := handlerInst.projectAgentRepo.GetByProject(ctx, req.Provider, req.Repo)
	if err != nil {
		hlog.Errorf("get project agents failed: %v", err)
		common.Fail(c, err)
		return
	}

	allAgents, err := handlerInst.reviewerAgentRepo.List(ctx)
	if err != nil {
		hlog.Errorf("get all agents failed: %v", err)
		common.Fail(c, err)
		return
	}

	agentMap := make(map[uint]models.ReviewerAgent)
	for _, a := range allAgents {
		agentMap[a.ID] = a
	}

	enabledMap := make(map[uint]models.ProjectAgent)
	for _, pa := range projectAgents {
		enabledMap[pa.AgentID] = pa
	}

	type AgentWithProjectConfig struct {
		AgentID         uint   `json:"agent_id"`
		Name            string `json:"name"`
		DisplayName     string `json:"display_name"`
		Description     string `json:"description"`
		Background      string `json:"background"`
		Enabled         bool   `json:"enabled"`
		Priority        int    `json:"priority"`
		ProjectEnabled  bool   `json:"project_enabled"`
		ProjectPriority int    `json:"project_priority"`
	}

	agents := slicex.Map(allAgents, func(a models.ReviewerAgent) AgentWithProjectConfig {
		agent := AgentWithProjectConfig{
			AgentID:         a.ID,
			Name:            a.Name,
			DisplayName:     a.DisplayName,
			Description:     a.Description,
			Background:      a.Background,
			Enabled:         a.Enabled,
			Priority:        a.Priority,
			ProjectEnabled:  false,
			ProjectPriority: a.Priority,
		}
		if pa, ok := enabledMap[a.ID]; ok {
			agent.ProjectEnabled = pa.Enabled
			agent.ProjectPriority = pa.Priority
		}
		return agent
	})

	c.JSON(http.StatusOK, map[string]interface{}{
		"provider": req.Provider,
		"repo":     req.Repo,
		"agents":   agents,
	})
}

type UpdateProjectAgentRequest struct {
	Provider string `json:"provider" binding:"required"`
	Repo     string `json:"repo" binding:"required"`
	AgentID  uint   `json:"agent_id" binding:"required"`
	Enabled  bool   `json:"enabled"`
	Priority int    `json:"priority"`
}

func UpdateProjectAgent(ctx context.Context, c *app.RequestContext) {
	var req modelProjectAgent.UpdateProjectAgentReq
	if err := c.BindAndValidate(&req); err != nil {
		common.Fail(c, common.BadRequest(err.Error()))
		return
	}

	_, err := handlerInst.reviewerAgentRepo.GetByID(ctx, uint(req.AgentID))
	if err != nil {
		common.Fail(c, common.NotFound("agent not found"))
		return
	}

	pa := &models.ProjectAgent{
		Provider: req.Provider,
		Repo:     req.Repo,
		AgentID:  uint(req.AgentID),
		Enabled:  req.Enabled,
		Priority: int(req.Priority),
	}

	if err := handlerInst.projectAgentRepo.Upsert(ctx, pa); err != nil {
		hlog.Errorf("upsert project agent failed: %v", err)
		common.Fail(c, err)
		return
	}

	common.Success(c, &modelProjectAgent.UpdateProjectAgentResp{
		Message: "project agent updated",
	})
}

type BatchUpdateProjectAgentsRequest struct {
	Provider string                   `json:"provider" binding:"required"`
	Repo     string                   `json:"repo" binding:"required"`
	Agents   []UpdateProjectAgentData `json:"agents" binding:"required"`
}

type UpdateProjectAgentData struct {
	AgentID  uint `json:"agent_id" binding:"required"`
	Enabled  bool `json:"enabled"`
	Priority int  `json:"priority"`
}

func BatchUpdateProjectAgents(ctx context.Context, c *app.RequestContext) {
	var req modelProjectAgent.BatchUpdateProjectAgentsReq
	if err := c.BindAndValidate(&req); err != nil {
		common.Fail(c, common.BadRequest(err.Error()))
		return
	}

	if err := handlerInst.projectAgentRepo.DeleteByProject(ctx, req.Provider, req.Repo); err != nil {
		hlog.Errorf("delete project agents failed: %v", err)
		common.Fail(c, err)
		return
	}

	var agentData []struct {
		AgentID  uint `json:"agent_id"`
		Enabled  bool `json:"enabled"`
		Priority int  `json:"priority"`
	}
	if err := json.Unmarshal([]byte(req.Agents), &agentData); err != nil {
		common.Fail(c, common.BadRequest("invalid agents format"))
		return
	}

	for _, agentReq := range agentData {
		pa := &models.ProjectAgent{
			Provider: req.Provider,
			Repo:     req.Repo,
			AgentID:  agentReq.AgentID,
			Enabled:  agentReq.Enabled,
			Priority: agentReq.Priority,
		}
		if err := handlerInst.projectAgentRepo.Upsert(ctx, pa); err != nil {
			hlog.Errorf("upsert project agent failed: %v", err)
		}
	}

	common.Success(c, &modelProjectAgent.BatchUpdateProjectAgentsResp{
		Message: "project agents updated",
		Count:   int64(len(agentData)),
	})
}

type DeleteProjectAgentRequest struct {
	Provider string `json:"provider" binding:"required"`
	Repo     string `json:"repo" binding:"required"`
	AgentID  uint   `json:"agent_id" binding:"required"`
}

func DeleteProjectAgent(ctx context.Context, c *app.RequestContext) {
	var req modelProjectAgent.DeleteProjectAgentReq
	if err := c.BindAndValidate(&req); err != nil {
		common.Fail(c, common.BadRequest(err.Error()))
		return
	}

	if err := handlerInst.projectAgentRepo.Delete(ctx, req.Provider, req.Repo, uint(req.AgentID)); err != nil {
		hlog.Errorf("delete project agent failed: %v", err)
		common.Fail(c, err)
		return
	}

	common.Success(c, &modelProjectAgent.DeleteProjectAgentResp{
		Message: "project agent deleted",
	})
}

func GetProjectAgentConfig(ctx context.Context, provider, repo string) ([]models.ProjectAgent, error) {
	return handlerInst.projectAgentRepo.GetEnabledByProject(ctx, provider, repo)
}

func uintPtr(v uint) *uint {
	return ptrx.Of(v)
}
