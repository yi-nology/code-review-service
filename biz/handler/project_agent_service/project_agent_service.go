package project_agent_service

import (
	"context"

	"github.com/yi-nology/code-review-service/biz/handler/common"
	modelProjectAgent "github.com/yi-nology/code-review-service/biz/model/project_agent"
	"github.com/yi-nology/code-review-service/internal/repository"
	"github.com/yi-nology/code-review-service/internal/repository/postgres/models"
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
	req, ok := common.Bind[modelProjectAgent.GetProjectAgentsReq](c)
	if !ok { return }

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

	enabledMap := make(map[uint]models.ProjectAgent)
	for _, pa := range projectAgents {
		enabledMap[pa.AgentID] = pa
	}

	agents := slicex.Map(allAgents, func(a models.ReviewerAgent) *modelProjectAgent.ProjectAgentConfig {
		agent := &modelProjectAgent.ProjectAgentConfig{
			AgentID:         common.SafeInt64(a.ID),
			Name:            a.Name,
			DisplayName:     a.DisplayName,
			Description:     a.Description,
			Background:      a.Background,
			Enabled:         a.Enabled,
			Priority:        int64(a.Priority),
			ProjectEnabled:  false,
			ProjectPriority: int64(a.Priority),
		}
		if pa, ok := enabledMap[a.ID]; ok {
			agent.ProjectEnabled = pa.Enabled
			agent.ProjectPriority = int64(pa.Priority)
		}
		return agent
	})

	common.Success(c, &modelProjectAgent.GetProjectAgentsResp{
		Provider: req.Provider,
		Repo:     req.Repo,
		Agents:   agents,
	})
}

func UpdateProjectAgent(ctx context.Context, c *app.RequestContext) {
	req, ok := common.Bind[modelProjectAgent.UpdateProjectAgentReq](c)
	if !ok { return }

	_, err := handlerInst.reviewerAgentRepo.GetByID(ctx, common.SafeUint(req.AgentID))
	if err != nil {
		common.Fail(c, common.NotFound("agent not found"))
		return
	}

	pa := &models.ProjectAgent{
		Provider: req.Provider,
		Repo:     req.Repo,
		AgentID:  common.SafeUint(req.AgentID),
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

func BatchUpdateProjectAgents(ctx context.Context, c *app.RequestContext) {
	req, ok := common.Bind[modelProjectAgent.BatchUpdateProjectAgentsReq](c)
	if !ok { return }

	if err := handlerInst.projectAgentRepo.DeleteByProject(ctx, req.Provider, req.Repo); err != nil {
		hlog.Errorf("delete project agents failed: %v", err)
		common.Fail(c, err)
		return
	}

	for _, agentReq := range req.Agents {
		pa := &models.ProjectAgent{
			Provider: req.Provider,
			Repo:     req.Repo,
			AgentID:  common.SafeUint(agentReq.AgentID),
			Enabled:  agentReq.Enabled,
			Priority: int(agentReq.Priority),
		}
		if err := handlerInst.projectAgentRepo.Upsert(ctx, pa); err != nil {
			hlog.Errorf("upsert project agent failed: %v", err)
		}
	}

	common.Success(c, &modelProjectAgent.BatchUpdateProjectAgentsResp{
		Message: "project agents updated",
		Count:   int64(len(req.Agents)),
	})
}

func DeleteProjectAgent(ctx context.Context, c *app.RequestContext) {
	req, ok := common.Bind[modelProjectAgent.DeleteProjectAgentReq](c)
	if !ok { return }

	if err := handlerInst.projectAgentRepo.Delete(ctx, req.Provider, req.Repo, common.SafeUint(req.AgentID)); err != nil {
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
