package settings_service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/yi-nology/code-review-service/biz/handler/common"
	modelSettings "github.com/yi-nology/code-review-service/biz/model/settings"
	svcApp "github.com/yi-nology/code-review-service/internal/app"
	"github.com/yi-nology/code-review-service/internal/repository"
	"github.com/yi-nology/code-review-service/internal/repository/postgres/models"
	"github.com/yi-nology/code-review-service/internal/service/review"

	"github.com/oekit/pkg/convert"
	"github.com/oekit/pkg/ptrx"
	"github.com/oekit/pkg/slicex"

	"github.com/cloudwego/hertz/pkg/app"
	"gorm.io/datatypes"
)

var handlerInst *SettingsHandler

type SettingsHandler struct {
	ocrRepo     repository.OCRSettingsRepository
	project     repository.ProjectPolicyRepository
	platform    repository.GitPlatformRepository
	globalRules repository.GlobalRulesRepository
	agentRepo   repository.ReviewerAgentRepository
	gitMgr      svcApp.GitProviderManager
	llmTester   review.LLMTester
}

func InitHandler(
	ocrRepo repository.OCRSettingsRepository,
	project repository.ProjectPolicyRepository,
	platform repository.GitPlatformRepository,
	globalRules repository.GlobalRulesRepository,
	agentRepo repository.ReviewerAgentRepository,
	gitMgr svcApp.GitProviderManager,
	llmTester review.LLMTester,
) {
	handlerInst = &SettingsHandler{
		ocrRepo:     ocrRepo,
		project:     project,
		platform:    platform,
		globalRules: globalRules,
		agentRepo:   agentRepo,
		gitMgr:      gitMgr,
		llmTester:   llmTester,
	}
}

// ==================== OCR Settings ====================

func GetOCRSettings(ctx context.Context, c *app.RequestContext) {
	settings, err := handlerInst.ocrRepo.Get(ctx)
	if err != nil {
		common.Fail(c, err)
		return
	}

	resp := dbOCRSettingsToAPI(settings)
	if ptrx.Deref(resp.AuthToken, "") != "" {
		resp.AuthToken = common.StrPtr("***")
	}

	common.Success(c, &modelSettings.GetOCRSettingsResp{Settings: resp})
}

func UpdateOCRSettings(ctx context.Context, c *app.RequestContext) {
	var req modelSettings.UpdateOCRSettingsReq
	if err := c.BindAndValidate(&req); err != nil {
		common.Fail(c, common.BadRequest(err.Error()))
		return
	}

	dbSettings := apiOCRSettingsToDB(req.Settings)
	if err := validateOCRSettings(dbSettings); err != nil {
		common.Fail(c, common.BadRequest(err.Error()))
		return
	}
	if dbSettings.AuthToken == "" || dbSettings.AuthToken == "***" {
		existing, err := handlerInst.ocrRepo.Get(ctx)
		if err == nil && existing != nil {
			dbSettings.AuthToken = existing.AuthToken
		}
	}

	if err := handlerInst.ocrRepo.Save(ctx, dbSettings); err != nil {
		common.Fail(c, common.BadRequest(err.Error()))
		return
	}

	resp := dbOCRSettingsToAPI(dbSettings)
	if ptrx.Deref(resp.AuthToken, "") != "" {
		resp.AuthToken = common.StrPtr("***")
	}

	common.Success(c, &modelSettings.UpdateOCRSettingsResp{Settings: resp})
}

func validateOCRSettings(s *models.OCRSettings) error {
	if s == nil {
		return fmt.Errorf("OCR 配置不能为空")
	}
	if strings.TrimSpace(s.Binary) == "" {
		s.Binary = "ocr"
	}
	if strings.TrimSpace(s.Audience) == "" {
		s.Audience = "human"
	}
	if s.Audience != "human" && s.Audience != "agent" {
		return fmt.Errorf("audience 仅支持 human 或 agent")
	}
	if strings.TrimSpace(s.DefaultSeverity) == "" {
		s.DefaultSeverity = "major"
	}
	switch s.DefaultSeverity {
	case "critical", "major", "minor", "info":
	default:
		return fmt.Errorf("默认严重级别仅支持 critical、major、minor、info")
	}
	if s.Concurrency < 0 || s.Concurrency > 32 {
		return fmt.Errorf("模型并发数需在 0 到 32 之间")
	}
	if s.MaxGitProcs < 0 || s.MaxGitProcs > 64 {
		return fmt.Errorf("Git 并发数需在 0 到 64 之间")
	}
	if s.TimeoutMinutes < 0 || s.TimeoutMinutes > 120 {
		return fmt.Errorf("超时时间需在 0 到 120 分钟之间")
	}
	if s.MaxTools < 0 || s.MaxTools > 100 {
		return fmt.Errorf("最大工具轮次需在 0 到 100 之间")
	}
	if len(s.ExtraBody) > 0 && !json.Valid(s.ExtraBody) {
		return fmt.Errorf("额外请求体必须是合法 JSON")
	}
	return nil
}

func TestLLM(ctx context.Context, c *app.RequestContext) {
	if handlerInst.llmTester == nil {
		common.Fail(c, common.BadRequest("LLM 测试功能未配置"))
		return
	}

	result, err := handlerInst.llmTester.TestLLM(ctx)
	if err != nil {
		common.Success(c, &modelSettings.TestLLMResp{
			Result: &modelSettings.TestLLMResult{Success: false, Message: common.StrPtr(err.Error())},
		})
		return
	}

	common.Success(c, &modelSettings.TestLLMResp{
		Result: &modelSettings.TestLLMResult{Success: result.Success, Message: common.StrPtr(result.Message)},
	})
}

// ==================== Project Policies ====================

func ListProjectPolicies(ctx context.Context, c *app.RequestContext) {
	items, err := handlerInst.project.List(ctx)
	if err != nil {
		common.Fail(c, err)
		return
	}

	policies := slicex.Map(items, func(item models.ProjectPolicy) *modelSettings.ProjectPolicy {
		return dbProjectPolicyToAPI(&item)
	})

	common.Success(c, &modelSettings.ListProjectPoliciesResp{Items: policies})
}

func PutProjectPolicy(ctx context.Context, c *app.RequestContext) {
	var req modelSettings.PutProjectPolicyReq
	if err := c.BindAndValidate(&req); err != nil {
		common.Fail(c, common.BadRequest(err.Error()))
		return
	}

	dbPolicy := apiProjectPolicyToDB(req.Policy)
	if err := handlerInst.project.Save(ctx, dbPolicy); err != nil {
		common.Fail(c, common.BadRequest(err.Error()))
		return
	}

	common.Success(c, &modelSettings.PutProjectPolicyResp{Policy: dbProjectPolicyToAPI(dbPolicy)})
}

func DeleteProjectPolicy(ctx context.Context, c *app.RequestContext) {
	var req modelSettings.DeleteProjectPolicyReq
	if err := c.BindAndValidate(&req); err != nil {
		common.Fail(c, common.BadRequest(err.Error()))
		return
	}
	if req.Provider == "" || req.Repo == "" {
		common.Fail(c, common.BadRequest("代码平台和仓库不能为空"))
		return
	}
	if err := handlerInst.project.Delete(ctx, req.Provider, req.Repo); err != nil {
		common.Fail(c, err)
		return
	}
	common.Success(c, &modelSettings.DeleteProjectPolicyResp{})
}

// ==================== Platforms ====================

func ListPlatforms(ctx context.Context, c *app.RequestContext) {
	items, err := handlerInst.platform.List(ctx)
	if err != nil {
		common.Fail(c, err)
		return
	}

	platforms := slicex.Map(items, func(item models.GitPlatform) *modelSettings.GitPlatformInfo {
		return &modelSettings.GitPlatformInfo{
			Provider: item.Provider, BaseURL: item.BaseURL,
			Token: common.MaskToken(item.Token), Enabled: item.Enabled,
			WebhookSecret:  common.MaskWebhookSecret(item.WebhookSecret),
			InstallationID: &item.InstallationID,
		}
	})

	common.Success(c, &modelSettings.ListPlatformsResp{Items: platforms})
}

func PutPlatform(ctx context.Context, c *app.RequestContext) {
	var req modelSettings.PutPlatformReq
	if err := c.BindAndValidate(&req); err != nil {
		common.Fail(c, common.BadRequest(err.Error()))
		return
	}
	if req.Provider == "" {
		common.Fail(c, common.BadRequest("代码平台不能为空"))
		return
	}

	token := ptrx.Deref(req.Token, "")
	if token == "" {
		existing, err := handlerInst.platform.Get(ctx, req.Provider)
		if err == nil && existing != nil {
			token = existing.Token
		}
	}
	if token == "" {
		common.Fail(c, common.BadRequest("Token 不能为空"))
		return
	}

	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	p := &models.GitPlatform{
		Provider: req.Provider, BaseURL: ptrx.Deref(req.BaseURL, ""),
		Token: token, WebhookSecret: ptrx.Deref(req.WebhookSecret, ""), Enabled: enabled,
		InstallationID: ptrx.Deref(req.InstallationID, ""),
	}
	if err := handlerInst.platform.Save(ctx, p); err != nil {
		common.Fail(c, common.BizErr(500, "保存平台配置失败"))
		return
	}

	if handlerInst.gitMgr != nil {
		if enabled {
			_ = handlerInst.gitMgr.AddOrUpdateFromDB(p)
		} else {
			handlerInst.gitMgr.Remove(p.Provider)
		}
	}

	common.Success(c, &modelSettings.PutPlatformResp{
		Platform: &modelSettings.GitPlatformInfo{
			Provider: p.Provider, BaseURL: p.BaseURL,
			Token: common.MaskToken(p.Token), Enabled: p.Enabled,
			WebhookSecret: common.MaskWebhookSecret(p.WebhookSecret),
		},
	})
}

func DeletePlatform(ctx context.Context, c *app.RequestContext) {
	var req modelSettings.DeletePlatformReq
	if err := c.BindAndValidate(&req); err != nil {
		common.Fail(c, common.BadRequest(err.Error()))
		return
	}
	if req.Provider == "" {
		common.Fail(c, common.BadRequest("代码平台不能为空"))
		return
	}
	if err := handlerInst.platform.Delete(ctx, req.Provider); err != nil {
		common.Fail(c, err)
		return
	}
	if handlerInst.gitMgr != nil {
		handlerInst.gitMgr.Remove(req.Provider)
	}
	common.Success(c, &modelSettings.DeletePlatformResp{})
}

func TestPlatform(ctx context.Context, c *app.RequestContext) {
	var req modelSettings.TestPlatformReq
	if err := c.BindAndValidate(&req); err != nil {
		common.Fail(c, common.BadRequest(err.Error()))
		return
	}
	if req.Provider == "" || req.Token == "" {
		common.Fail(c, common.BadRequest("平台和 Token 不能为空"))
		return
	}

	result, err := handlerInst.gitMgr.TestWithToken(ctx, req.Provider, ptrx.Deref(req.BaseURL, ""), req.Token)
	if err != nil {
		common.Success(c, &modelSettings.TestPlatformResp{
			Result: &modelSettings.GitTestResult{Connected: false, Platform: req.Provider, Message: common.StrPtr(err.Error())},
		})
		return
	}
	common.Success(c, &modelSettings.TestPlatformResp{
		Result: &modelSettings.GitTestResult{Connected: result.Connected, Platform: result.Platform, UserName: common.StrPtr(result.UserName), Message: common.StrPtr(result.Message)},
	})
}

// ==================== Global Rules ====================

func GetGlobalRules(ctx context.Context, c *app.RequestContext) {
	rules, err := handlerInst.globalRules.Get(ctx)
	if err != nil {
		common.Fail(c, err)
		return
	}

	resp := dbGlobalRulesToAPI(rules)
	common.Success(c, &modelSettings.GetGlobalRulesResp{Rules: resp})
}

func UpdateGlobalRules(ctx context.Context, c *app.RequestContext) {
	var req modelSettings.UpdateGlobalRulesReq
	if err := c.BindAndValidate(&req); err != nil {
		common.Fail(c, common.BadRequest(err.Error()))
		return
	}

	dbRules := apiGlobalRulesToDB(req.Rules)
	if err := handlerInst.globalRules.Save(ctx, dbRules); err != nil {
		common.Fail(c, common.BizErr(500, "保存全局规则失败"))
		return
	}

	common.Success(c, &modelSettings.UpdateGlobalRulesResp{Rules: dbGlobalRulesToAPI(dbRules)})
}

// ==================== Agents ====================

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
	var req modelSettings.CreateReviewerAgentReq
	if err := c.BindAndValidate(&req); err != nil {
		common.Fail(c, common.BadRequest(err.Error()))
		return
	}

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

	common.Success(c, &modelSettings.CreateReviewerAgentResp{
		Agent: dbReviewerAgentToAPI(agent),
	})
}

func UpdateReviewerAgent(ctx context.Context, c *app.RequestContext) {
	var req modelSettings.UpdateReviewerAgentReq
	if err := c.BindAndValidate(&req); err != nil {
		common.Fail(c, common.BadRequest(err.Error()))
		return
	}

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

	common.Success(c, &modelSettings.UpdateReviewerAgentResp{
		Agent: dbReviewerAgentToAPI(existing),
	})
}

func DeleteReviewerAgent(ctx context.Context, c *app.RequestContext) {
	var req modelSettings.DeleteReviewerAgentReq
	if err := c.BindAndValidate(&req); err != nil {
		common.Fail(c, common.BadRequest(err.Error()))
		return
	}

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

// ==================== Helpers ====================

func parseUintID(s string) (uint, error) {
	id := convert.Str2Int(s, -1)
	if id < 0 {
		return 0, fmt.Errorf("invalid id: %s", s)
	}
	return uint(id), nil
}

func mustMarshalJSON(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}

// ==================== Converters ====================

func dbOCRSettingsToAPI(s *models.OCRSettings) *modelSettings.OCRSettings {
	var extraBody string
	if len(s.ExtraBody) > 0 {
		extraBody = string(s.ExtraBody)
	}
	return &modelSettings.OCRSettings{
		Enabled: s.Enabled, Provider: common.StrPtr(s.Provider), Plan: common.StrPtr(s.Plan),
		URL: common.StrPtr(s.URL), AuthToken: common.StrPtr(s.AuthToken), AuthHeader: common.StrPtr(s.AuthHeader),
		Model: common.StrPtr(s.ModelName), UseAnthropic: ptrx.Of(s.UseAnthropic), ExtraBody: common.StrPtr(extraBody),
		Language: common.StrPtr(s.Language), Concurrency: ptrx.Of(int64(s.Concurrency)),
		MaxGitProcs: ptrx.Of(int64(s.MaxGitProcs)), TimeoutMinutes: ptrx.Of(int64(s.TimeoutMinutes)),
		MaxTools: ptrx.Of(int64(s.MaxTools)), Background: common.StrPtr(s.Background),
		Binary: common.StrPtr(s.Binary), Audience: common.StrPtr(s.Audience),
		DefaultSeverity: common.StrPtr(s.DefaultSeverity),
	}
}

func apiOCRSettingsToDB(s *modelSettings.OCRSettings) *models.OCRSettings {
	db := &models.OCRSettings{}
	if s == nil {
		return db
	}
	db.Enabled = s.Enabled
	db.Binary = ptrx.Deref(s.Binary, "")
	db.Audience = ptrx.Deref(s.Audience, "")
	db.DefaultSeverity = ptrx.Deref(s.DefaultSeverity, "")
	db.Provider = ptrx.Deref(s.Provider, "")
	db.Plan = ptrx.Deref(s.Plan, "")
	db.URL = ptrx.Deref(s.URL, "")
	db.AuthToken = ptrx.Deref(s.AuthToken, "")
	db.AuthHeader = ptrx.Deref(s.AuthHeader, "")
	db.ModelName = ptrx.Deref(s.Model, "")
	db.UseAnthropic = ptrx.Deref(s.UseAnthropic, false)
	db.Language = ptrx.Deref(s.Language, "")
	db.Concurrency = int(ptrx.Deref(s.Concurrency, 0))
	db.MaxGitProcs = int(ptrx.Deref(s.MaxGitProcs, 0))
	db.TimeoutMinutes = int(ptrx.Deref(s.TimeoutMinutes, 0))
	db.MaxTools = int(ptrx.Deref(s.MaxTools, 0))
	db.Background = ptrx.Deref(s.Background, "")
	if s.IsSetExtraBody() && s.ExtraBody != nil && *s.ExtraBody != "" {
		db.ExtraBody = datatypes.JSON(*s.ExtraBody)
	}
	return db
}

func dbProjectPolicyToAPI(p *models.ProjectPolicy) *modelSettings.ProjectPolicy {
	var ignorePaths, rules string
	if len(p.IgnorePaths) > 0 {
		ignorePaths = string(p.IgnorePaths)
	}
	if len(p.Rules) > 0 {
		rules = string(p.Rules)
	}
	return &modelSettings.ProjectPolicy{
		Provider: p.Provider, Repo: p.Repo, FailOn: p.FailOn,
		IgnorePaths: common.StrPtr(ignorePaths), Rules: common.StrPtr(rules),
	}
}

func apiProjectPolicyToDB(p *modelSettings.ProjectPolicy) *models.ProjectPolicy {
	db := &models.ProjectPolicy{}
	if p == nil {
		return db
	}
	db.Provider = p.Provider
	db.Repo = p.Repo
	db.FailOn = p.FailOn
	if p.IsSetIgnorePaths() && p.IgnorePaths != nil && *p.IgnorePaths != "" {
		db.IgnorePaths = datatypes.JSON(*p.IgnorePaths)
	}
	if p.IsSetRules() && p.Rules != nil && *p.Rules != "" {
		db.Rules = datatypes.JSON(*p.Rules)
	}
	return db
}

func dbGlobalRulesToAPI(r *models.GlobalRules) *modelSettings.GlobalRules {
	if r == nil {
		return &modelSettings.GlobalRules{}
	}
	var entries []*modelSettings.ReviewRule
	if len(r.Rules) > 0 {
		var rules []struct {
			Path string `json:"path"`
			Rule string `json:"rule"`
		}
		if err := json.Unmarshal(r.Rules, &rules); err == nil {
			for _, r := range rules {
				entries = append(entries, &modelSettings.ReviewRule{Path: r.Path, Rule: r.Rule})
			}
		}
	}
	return &modelSettings.GlobalRules{Rules: entries}
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
		ID: int64(a.ID), Name: a.Name, DisplayName: a.DisplayName,
		Description: common.StrPtr(a.Description), Background: common.StrPtr(a.Background),
		Enabled: a.Enabled, Priority: int64(a.Priority), Tags: common.StrPtr(tags),
	}
}

func apiGlobalRulesToDB(g *modelSettings.GlobalRules) *models.GlobalRules {
	db := &models.GlobalRules{}
	if g == nil {
		return db
	}
	type ruleEntry struct {
		Path string `json:"path"`
		Rule string `json:"rule"`
	}
	entries := slicex.Map(g.Rules, func(r *modelSettings.ReviewRule) ruleEntry {
		return ruleEntry{Path: r.Path, Rule: r.Rule}
	})
	data, _ := json.Marshal(entries)
	db.Rules = datatypes.JSON(data)
	return db
}
