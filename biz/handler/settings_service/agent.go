package settings_service

import (
	"context"
	"encoding/json"

	"github.com/yi-nology/code-review-service/biz/handler/common"
	modelSettings "github.com/yi-nology/code-review-service/biz/model/settings"
	"github.com/yi-nology/code-review-service/internal/domain/reviewer"

	"github.com/oekit/pkg/ptrx"
	"github.com/oekit/pkg/slicex"

	"github.com/cloudwego/hertz/pkg/app"
)

func (h *SettingsHandler) listReviewerAgents(ctx context.Context, c *app.RequestContext) {
	agents, err := h.agentRepo.List(ctx)
	if err != nil {
		common.Fail(ctx, c, err)
		return
	}
	items := slicex.Map(agents, func(a reviewer.Agent) *modelSettings.ReviewerAgentInfo {
		return dbReviewerAgentToAPI(&a)
	})
	common.Success(c, &modelSettings.ListReviewerAgentsResp{Agents: items})
}

// ListReviewerAgents 包级入口（biz/router 生成代码引用）；一行委托到方法，测试可直接构造 Handler。
func ListReviewerAgents(ctx context.Context, c *app.RequestContext) {
	handlerInst.listReviewerAgents(ctx, c)
}

func (h *SettingsHandler) createReviewerAgent(ctx context.Context, c *app.RequestContext) {
	req, ok := common.Bind[modelSettings.CreateReviewerAgentReq](c)
	if !ok {
		return
	}
	agent := &reviewer.Agent{
		Name: req.Name, DisplayName: req.DisplayName, Description: ptrx.Deref(req.Description, ""),
		Background: req.Background, Enabled: true, Redundancy: 1,
	}
	if req.Enabled != nil {
		agent.Enabled = *req.Enabled
	}
	if req.Priority != nil {
		agent.Priority = int(*req.Priority)
	}
	if req.RuleFile != nil {
		agent.RuleFile = *req.RuleFile
	}
	if req.Redundancy != nil && *req.Redundancy >= 1 {
		agent.Redundancy = int(*req.Redundancy)
	}
	if req.IsDefault != nil {
		agent.IsDefault = *req.IsDefault
	}
	if len(req.Tags) > 0 {
		agent.Tags = json.RawMessage(mustMarshalJSON(req.Tags))
	}
	if len(req.RulePackageIds) > 0 {
		ids := make([]uint, 0, len(req.RulePackageIds))
		for _, id := range req.RulePackageIds {
			ids = append(ids, uint(id))
		}
		agent.RulePackageIDs = json.RawMessage(mustMarshalJSON(ids))
	}
	if err := h.agentRepo.Create(ctx, agent); err != nil {
		common.Fail(ctx, c, err)
		return
	}
	common.Success(c, &modelSettings.CreateReviewerAgentResp{Agent: dbReviewerAgentToAPI(agent)})
}

// CreateReviewerAgent 包级入口（biz/router 生成代码引用）；一行委托到方法，测试可直接构造 Handler。
func CreateReviewerAgent(ctx context.Context, c *app.RequestContext) {
	handlerInst.createReviewerAgent(ctx, c)
}

func (h *SettingsHandler) updateReviewerAgent(ctx context.Context, c *app.RequestContext) {
	req, ok := common.Bind[modelSettings.UpdateReviewerAgentReq](c)
	if !ok {
		return
	}
	id, err := common.ParseUintID(req.ID)
	if err != nil {
		common.Fail(ctx, c, common.BadRequest("invalid agent id"))
		return
	}
	existing, err := h.agentRepo.GetByID(ctx, id)
	if err != nil {
		common.Fail(ctx, c, common.NotFound("agent not found"))
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
	if req.RuleFile != nil {
		existing.RuleFile = *req.RuleFile
	}
	if req.Redundancy != nil && *req.Redundancy >= 1 {
		existing.Redundancy = int(*req.Redundancy)
	}
	if req.IsDefault != nil {
		existing.IsDefault = *req.IsDefault
	}
	if req.Tags != nil {
		existing.Tags = json.RawMessage(mustMarshalJSON(req.Tags))
	}
	// RulePackageIDs：optional，nil 表示不修改；非 nil（含空数组）表示覆盖。
	if req.RulePackageIds != nil {
		ids := make([]uint, 0, len(req.RulePackageIds))
		for _, id := range req.RulePackageIds {
			ids = append(ids, uint(id))
		}
		existing.RulePackageIDs = json.RawMessage(mustMarshalJSON(ids))
	}
	if err := h.agentRepo.Update(ctx, existing); err != nil {
		common.Fail(ctx, c, err)
		return
	}
	common.Success(c, &modelSettings.UpdateReviewerAgentResp{Agent: dbReviewerAgentToAPI(existing)})
}

// UpdateReviewerAgent 包级入口（biz/router 生成代码引用）；一行委托到方法，测试可直接构造 Handler。
func UpdateReviewerAgent(ctx context.Context, c *app.RequestContext) {
	handlerInst.updateReviewerAgent(ctx, c)
}

func (h *SettingsHandler) deleteReviewerAgent(ctx context.Context, c *app.RequestContext) {
	req, ok := common.Bind[modelSettings.DeleteReviewerAgentReq](c)
	if !ok {
		return
	}
	id, err := common.ParseUintID(req.ID)
	if err != nil {
		common.Fail(ctx, c, common.BadRequest("invalid agent id"))
		return
	}
	if err := h.agentRepo.Delete(ctx, id); err != nil {
		common.Fail(ctx, c, err)
		return
	}
	common.Success(c, &modelSettings.DeleteReviewerAgentResp{})
}

// DeleteReviewerAgent 包级入口（biz/router 生成代码引用）；一行委托到方法，测试可直接构造 Handler。
func DeleteReviewerAgent(ctx context.Context, c *app.RequestContext) {
	handlerInst.deleteReviewerAgent(ctx, c)
}

func dbReviewerAgentToAPI(a *reviewer.Agent) *modelSettings.ReviewerAgentInfo {
	if a == nil {
		return nil
	}
	var tags string
	if len(a.Tags) > 0 {
		tags = string(a.Tags)
	}
	return &modelSettings.ReviewerAgentInfo{
		ID: common.SafeInt64(a.ID), Name: a.Name, DisplayName: a.DisplayName,
		Description: common.StrPtr(a.Description), Background: common.StrPtr(a.Background),
		Enabled: a.Enabled, Priority: int64(a.Priority), Tags: common.StrPtr(tags),
		RuleFile: common.StrPtr(a.RuleFile), Redundancy: int64(a.Redundancy),
		IsDefault:      a.IsDefault,
		RulePackageIds: rulePackageIDsToAPI(a.RulePackageIDs),
	}
}

// rulePackageIDsToAPI 把审查员引用的规则包 id（json.RawMessage, uint 数组）转成 API 的 []int64。
// 解析失败或为空时返回 nil（前端按空数组处理）。
func rulePackageIDsToAPI(raw json.RawMessage) []int64 {
	if len(raw) == 0 {
		return nil
	}
	var ids []uint
	if err := json.Unmarshal(raw, &ids); err != nil {
		return nil
	}
	result := make([]int64, 0, len(ids))
	for _, id := range ids {
		result = append(result, int64(id))
	}
	return result
}
