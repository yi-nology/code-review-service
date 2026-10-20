package project_agent_service

import (
	"context"

	"github.com/oekit/pkg/slicex"
	"github.com/yi-nology/code-review-service/biz/handler/common"
	modelProjectAgent "github.com/yi-nology/code-review-service/biz/model/project_agent"
	"github.com/yi-nology/code-review-service/internal/domain/reviewer"
	"github.com/yi-nology/code-review-service/internal/repository"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/oekit/logx"
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

func (h *ProjectAgentHandler) getProjectAgents(ctx context.Context, c *app.RequestContext) {
	req, ok := common.Bind[modelProjectAgent.GetProjectAgentsReq](c)
	if !ok {
		return
	}

	projectAgents, err := h.projectAgentRepo.GetByProject(ctx, req.Provider, req.Repo)
	if err != nil {
		logx.Errorf("get project agents failed: %v", err)
		common.Fail(ctx, c, err)
		return
	}

	allAgents, err := h.reviewerAgentRepo.List(ctx)
	if err != nil {
		logx.Errorf("get all agents failed: %v", err)
		common.Fail(ctx, c, err)
		return
	}

	enabledMap := make(map[uint]reviewer.ProjectAssignment)
	for _, pa := range projectAgents {
		enabledMap[pa.AgentID] = pa
	}

	agents := slicex.Map(allAgents, func(a reviewer.Agent) *modelProjectAgent.ProjectAgentConfig {
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
			IsDefault:       a.IsDefault,
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

// GetProjectAgents 包级入口（biz/router 生成代码引用）；一行委托到方法，便于测试直接构造 Handler。
func GetProjectAgents(ctx context.Context, c *app.RequestContext) {
	handlerInst.getProjectAgents(ctx, c)
}

func (h *ProjectAgentHandler) updateProjectAgent(ctx context.Context, c *app.RequestContext) {
	req, ok := common.Bind[modelProjectAgent.UpdateProjectAgentReq](c)
	if !ok {
		return
	}

	_, err := h.reviewerAgentRepo.GetByID(ctx, common.SafeUint(req.AgentID))
	if err != nil {
		common.Fail(ctx, c, common.NotFound("agent not found"))
		return
	}

	pa := &reviewer.ProjectAssignment{
		Provider: req.Provider,
		Repo:     req.Repo,
		AgentID:  common.SafeUint(req.AgentID),
		Enabled:  req.Enabled,
		Priority: int(req.Priority),
	}

	if err := h.projectAgentRepo.Upsert(ctx, pa); err != nil {
		logx.Errorf("upsert project agent failed: %v", err)
		common.Fail(ctx, c, err)
		return
	}

	common.Success(c, &modelProjectAgent.UpdateProjectAgentResp{
		Message: "project agent updated",
	})
}

// UpdateProjectAgent 包级入口（biz/router 生成代码引用）；一行委托到方法，便于测试直接构造 Handler。
func UpdateProjectAgent(ctx context.Context, c *app.RequestContext) {
	handlerInst.updateProjectAgent(ctx, c)
}

func (h *ProjectAgentHandler) batchUpdateProjectAgents(ctx context.Context, c *app.RequestContext) {
	req, ok := common.Bind[modelProjectAgent.BatchUpdateProjectAgentsReq](c)
	if !ok {
		return
	}

	if err := h.projectAgentRepo.DeleteByProject(ctx, req.Provider, req.Repo); err != nil {
		logx.Errorf("delete project agents failed: %v", err)
		common.Fail(ctx, c, err)
		return
	}

	for _, agentReq := range req.Agents {
		pa := &reviewer.ProjectAssignment{
			Provider: req.Provider,
			Repo:     req.Repo,
			AgentID:  common.SafeUint(agentReq.AgentID),
			Enabled:  agentReq.Enabled,
			Priority: int(agentReq.Priority),
		}
		if err := h.projectAgentRepo.Upsert(ctx, pa); err != nil {
			logx.Errorf("upsert project agent failed: %v", err)
		}
	}

	common.Success(c, &modelProjectAgent.BatchUpdateProjectAgentsResp{
		Message: "project agents updated",
		Count:   int64(len(req.Agents)),
	})
}

// BatchUpdateProjectAgents 包级入口（biz/router 生成代码引用）；一行委托到方法，便于测试直接构造 Handler。
func BatchUpdateProjectAgents(ctx context.Context, c *app.RequestContext) {
	handlerInst.batchUpdateProjectAgents(ctx, c)
}

func (h *ProjectAgentHandler) deleteProjectAgent(ctx context.Context, c *app.RequestContext) {
	req, ok := common.Bind[modelProjectAgent.DeleteProjectAgentReq](c)
	if !ok {
		return
	}

	if err := h.projectAgentRepo.Delete(ctx, req.Provider, req.Repo, common.SafeUint(req.AgentID)); err != nil {
		logx.Errorf("delete project agent failed: %v", err)
		common.Fail(ctx, c, err)
		return
	}

	common.Success(c, &modelProjectAgent.DeleteProjectAgentResp{
		Message: "project agent deleted",
	})
}

// DeleteProjectAgent 包级入口（biz/router 生成代码引用）；一行委托到方法，便于测试直接构造 Handler。
func DeleteProjectAgent(ctx context.Context, c *app.RequestContext) {
	handlerInst.deleteProjectAgent(ctx, c)
}
