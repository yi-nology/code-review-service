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
	ocrRepo        repository.OCRSettingsRepository
	project        repository.ProjectPolicyRepository
	platform       repository.GitPlatformRepository
	globalRules    repository.GlobalRulesRepository
	agentRepo      repository.ReviewerAgentRepository
	customProvider repository.CustomProviderRepository
	gitMgr         svcApp.GitProviderManager
	llmTester      review.LLMTester
	providerLister review.LLMProviderLister
	rulesChecker   review.RulesChecker
	ocrVersioner   review.OCRVersioner
	viewerMgr      review.ViewerManager
	configSync     ConfigSyncer
}

type ConfigSyncer interface {
	Sync(ctx context.Context) error
}

func InitHandler(
	ocrRepo repository.OCRSettingsRepository,
	project repository.ProjectPolicyRepository,
	platform repository.GitPlatformRepository,
	globalRules repository.GlobalRulesRepository,
	agentRepo repository.ReviewerAgentRepository,
	customProvider repository.CustomProviderRepository,
	gitMgr svcApp.GitProviderManager,
	llmTester review.LLMTester,
	providerLister review.LLMProviderLister,
	rulesChecker review.RulesChecker,
	ocrVersioner review.OCRVersioner,
	viewerMgr review.ViewerManager,
	configSync ConfigSyncer,
) {
	handlerInst = &SettingsHandler{
		ocrRepo:        ocrRepo,
		project:         project,
		platform:        platform,
		globalRules:     globalRules,
		agentRepo:       agentRepo,
		customProvider:  customProvider,
		gitMgr:          gitMgr,
		llmTester:       llmTester,
		providerLister:  providerLister,
		rulesChecker:    rulesChecker,
		ocrVersioner:    ocrVersioner,
		viewerMgr:       viewerMgr,
		configSync:      configSync,
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

	if handlerInst.configSync != nil {
		_ = handlerInst.configSync.Sync(ctx)
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
	if err := validateProjectPolicy(dbPolicy); err != nil {
		common.Fail(c, common.BadRequest(err.Error()))
		return
	}
	if err := handlerInst.project.Save(ctx, dbPolicy); err != nil {
		common.Fail(c, common.BadRequest(err.Error()))
		return
	}

	common.Success(c, &modelSettings.PutProjectPolicyResp{Policy: dbProjectPolicyToAPI(dbPolicy)})
}

func validateProjectPolicy(p *models.ProjectPolicy) error {
	if p == nil {
		return fmt.Errorf("项目策略不能为空")
	}
	if strings.TrimSpace(p.Provider) == "" {
		return fmt.Errorf("代码平台不能为空")
	}
	if strings.TrimSpace(p.Repo) == "" {
		return fmt.Errorf("仓库不能为空")
	}
	switch p.FailOn {
	case "critical", "major", "minor", "info":
	default:
		return fmt.Errorf("门禁级别仅支持 critical、major、minor、info")
	}
	if p.OCRConcurrency < 0 || p.OCRConcurrency > 32 {
		return fmt.Errorf("项目 OCR 并发数需在 0 到 32 之间")
	}
	if p.OCRTimeoutMinutes < 0 || p.OCRTimeoutMinutes > 120 {
		return fmt.Errorf("项目 OCR 超时时间需在 0 到 120 分钟之间")
	}
	if p.OCRMaxTools < 0 || p.OCRMaxTools > 100 {
		return fmt.Errorf("项目 OCR 最大工具轮次需在 0 到 100 之间")
	}
	if strings.TrimSpace(p.OCRDefaultSeverity) != "" {
		switch p.OCRDefaultSeverity {
		case "critical", "major", "minor", "info":
		default:
			return fmt.Errorf("项目默认严重级别仅支持 critical、major、minor、info")
		}
	}
	return nil
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
		Enabled: s.Enabled, Provider: common.StrPtr(s.Provider),
		URL: common.StrPtr(s.URL), AuthToken: common.StrPtr(s.AuthToken), AuthHeader: common.StrPtr(s.AuthHeader),
		Model: common.StrPtr(s.ModelName), UseAnthropic: ptrx.Of(s.UseAnthropic), ExtraBody: common.StrPtr(extraBody),
		Language: common.StrPtr(s.Language), Concurrency: ptrx.Of(int64(s.Concurrency)),
		MaxGitProcs: ptrx.Of(int64(s.MaxGitProcs)), TimeoutMinutes: ptrx.Of(int64(s.TimeoutMinutes)),
		MaxTools: ptrx.Of(int64(s.MaxTools)), Background: common.StrPtr(s.Background),
		Binary: common.StrPtr(s.Binary), Audience: common.StrPtr(s.Audience),
		DefaultSeverity: common.StrPtr(s.DefaultSeverity),
		ToolsPath: common.StrPtr(s.ToolsPath),
		TelemetryEnabled: ptrx.Of(s.TelemetryEnabled), TelemetryExporter: common.StrPtr(s.TelemetryExporter),
		TelemetryOtlpEndpoint: common.StrPtr(s.TelemetryOTLPEndpoint), TelemetryContentLogging: ptrx.Of(s.TelemetryContentLogging),
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
	db.ToolsPath = ptrx.Deref(s.ToolsPath, "")
	db.TelemetryEnabled = ptrx.Deref(s.TelemetryEnabled, false)
	db.TelemetryExporter = ptrx.Deref(s.TelemetryExporter, "")
	db.TelemetryOTLPEndpoint = ptrx.Deref(s.TelemetryOtlpEndpoint, "")
	db.TelemetryContentLogging = ptrx.Deref(s.TelemetryContentLogging, false)
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
		OcrEnabled: p.OCREnabled, OcrConcurrency: ptrx.Of(int64(p.OCRConcurrency)),
		OcrTimeoutMinutes: ptrx.Of(int64(p.OCRTimeoutMinutes)), OcrMaxTools: ptrx.Of(int64(p.OCRMaxTools)),
		OcrDefaultSeverity: common.StrPtr(p.OCRDefaultSeverity), OcrBackground: common.StrPtr(p.OCRBackground),
		OcrAudience: common.StrPtr(p.OCRAudience), OcrProvider: common.StrPtr(p.OCRProvider),
		OcrURL: common.StrPtr(p.OCRURL), OcrModel: common.StrPtr(p.OCRModel),
		OcrMaxGitProcs: ptrx.Of(int64(p.OCRMaxGitProcs)), OcrUseAnthropic: ptrx.Of(p.OCRUseAnthropic),
		OcrExtraBody: common.StrPtr(p.OCRExtraBody), OcrLanguage: common.StrPtr(p.OCRLanguage),
		OcrBinary: common.StrPtr(p.OCRBinary), OcrToolsPath: common.StrPtr(p.OCRToolsPath),
		OcrTelemetryEnabled: ptrx.Of(p.OCRTelemetryEnabled),
		OcrTelemetryExporter: common.StrPtr(p.OCRTelemetryExporter),
		OcrTelemetryOtlpEndpoint: common.StrPtr(p.OCRTelemetryOTLPEndpoint),
		OcrTelemetryContentLogging: ptrx.Of(p.OCRTelemetryContentLogging),
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
	db.OCREnabled = p.OcrEnabled
	db.OCRConcurrency = int(ptrx.Deref(p.OcrConcurrency, 0))
	db.OCRTimeoutMinutes = int(ptrx.Deref(p.OcrTimeoutMinutes, 0))
	db.OCRMaxTools = int(ptrx.Deref(p.OcrMaxTools, 0))
	db.OCRDefaultSeverity = ptrx.Deref(p.OcrDefaultSeverity, "")
	db.OCRBackground = ptrx.Deref(p.OcrBackground, "")
	db.OCRAudience = ptrx.Deref(p.OcrAudience, "")
	db.OCRProvider = ptrx.Deref(p.OcrProvider, "")
	db.OCRURL = ptrx.Deref(p.OcrURL, "")
	db.OCRModel = ptrx.Deref(p.OcrModel, "")
	db.OCRMaxGitProcs = int(ptrx.Deref(p.OcrMaxGitProcs, 0))
	db.OCRUseAnthropic = ptrx.Deref(p.OcrUseAnthropic, false)
	db.OCRExtraBody = ptrx.Deref(p.OcrExtraBody, "")
	db.OCRLanguage = ptrx.Deref(p.OcrLanguage, "")
	db.OCRBinary = ptrx.Deref(p.OcrBinary, "")
	db.OCRToolsPath = ptrx.Deref(p.OcrToolsPath, "")
	db.OCRTelemetryEnabled = ptrx.Deref(p.OcrTelemetryEnabled, false)
	db.OCRTelemetryExporter = ptrx.Deref(p.OcrTelemetryExporter, "")
	db.OCRTelemetryOTLPEndpoint = ptrx.Deref(p.OcrTelemetryOtlpEndpoint, "")
	db.OCRTelemetryContentLogging = ptrx.Deref(p.OcrTelemetryContentLogging, false)
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

// ==================== LLM Providers ====================

func ListLLMProviders(ctx context.Context, c *app.RequestContext) {
	if handlerInst.providerLister == nil {
		common.Fail(c, common.BadRequest("LLM Provider 列表功能未配置"))
		return
	}

	providers, err := handlerInst.providerLister.ListLLMProviders(ctx)
	if err != nil {
		common.Fail(c, err)
		return
	}

	var items []*modelSettings.LLMProvider
	for _, p := range providers {
		item := &modelSettings.LLMProvider{
			Name:     p.Name,
			Protocol: p.Protocol,
		}
		if p.BaseURL != "" {
			item.BaseURL = common.StrPtr(p.BaseURL)
		}
		if len(p.DefaultModel) > 0 {
			item.DefaultModel = p.DefaultModel
		}
		items = append(items, item)
	}

	common.Success(c, &modelSettings.ListLLMProvidersResp{BuiltIn: items})
}

// ==================== Custom Providers ====================

func ListCustomProviders(ctx context.Context, c *app.RequestContext) {
	if handlerInst.customProvider == nil {
		common.Fail(c, common.BadRequest("自定义 Provider 功能未配置"))
		return
	}

	providers, err := handlerInst.customProvider.List(ctx)
	if err != nil {
		common.Fail(c, err)
		return
	}

	var items []*modelSettings.CustomProvider
	for i := range providers {
		items = append(items, dbCustomProviderToAPI(&providers[i]))
	}

	common.Success(c, &modelSettings.ListCustomProvidersResp{Items: items})
}

func CreateCustomProvider(ctx context.Context, c *app.RequestContext) {
	var req modelSettings.CreateCustomProviderReq
	if err := c.BindAndValidate(&req); err != nil {
		common.Fail(c, common.BadRequest(err.Error()))
		return
	}

	if strings.TrimSpace(req.Name) == "" {
		common.Fail(c, common.BadRequest("Provider 名称不能为空"))
		return
	}
	if strings.TrimSpace(req.Protocol) == "" {
		common.Fail(c, common.BadRequest("Protocol 不能为空"))
		return
	}

	p := &models.CustomProvider{
		Name:       req.Name,
		Protocol:   req.Protocol,
		URL:        ptrx.Deref(req.URL, ""),
		APIKey:     ptrx.Deref(req.APIKey, ""),
		ModelName:  ptrx.Deref(req.Model, ""),
		AuthHeader: ptrx.Deref(req.AuthHeader, ""),
		Enabled:    ptrx.Deref(req.Enabled, true),
	}
	if req.ExtraBody != nil && *req.ExtraBody != "" {
		p.ExtraBody = datatypes.JSON(*req.ExtraBody)
	}

	if err := handlerInst.customProvider.Create(ctx, p); err != nil {
		common.Fail(c, err)
		return
	}

	if handlerInst.configSync != nil {
		_ = handlerInst.configSync.Sync(ctx)
	}

	common.Success(c, &modelSettings.CreateCustomProviderResp{Provider: dbCustomProviderToAPI(p)})
}

func UpdateCustomProvider(ctx context.Context, c *app.RequestContext) {
	var req modelSettings.UpdateCustomProviderReq
	if err := c.BindAndValidate(&req); err != nil {
		common.Fail(c, common.BadRequest(err.Error()))
		return
	}

	existing, err := handlerInst.customProvider.GetByID(ctx, uint(req.ID))
	if err != nil || existing == nil {
		common.Fail(c, common.BadRequest("Provider 不存在"))
		return
	}

	if req.Name != nil {
		existing.Name = *req.Name
	}
	if req.Protocol != nil {
		existing.Protocol = *req.Protocol
	}
	if req.URL != nil {
		existing.URL = *req.URL
	}
	if req.APIKey != nil {
		existing.APIKey = *req.APIKey
	}
	if req.Model != nil {
		existing.ModelName = *req.Model
	}
	if req.AuthHeader != nil {
		existing.AuthHeader = *req.AuthHeader
	}
	if req.ExtraBody != nil && *req.ExtraBody != "" {
		existing.ExtraBody = datatypes.JSON(*req.ExtraBody)
	}
	if req.Enabled != nil {
		existing.Enabled = *req.Enabled
	}

	if err := handlerInst.customProvider.Update(ctx, existing); err != nil {
		common.Fail(c, err)
		return
	}

	if handlerInst.configSync != nil {
		_ = handlerInst.configSync.Sync(ctx)
	}

	common.Success(c, &modelSettings.UpdateCustomProviderResp{Provider: dbCustomProviderToAPI(existing)})
}

func DeleteCustomProvider(ctx context.Context, c *app.RequestContext) {
	var req modelSettings.DeleteCustomProviderReq
	if err := c.BindAndValidate(&req); err != nil {
		common.Fail(c, common.BadRequest(err.Error()))
		return
	}

	if err := handlerInst.customProvider.Delete(ctx, uint(req.ID)); err != nil {
		common.Fail(c, err)
		return
	}

	if handlerInst.configSync != nil {
		_ = handlerInst.configSync.Sync(ctx)
	}

	common.Success(c, &modelSettings.DeleteCustomProviderResp{})
}

func dbCustomProviderToAPI(p *models.CustomProvider) *modelSettings.CustomProvider {
	if p == nil {
		return nil
	}
	result := &modelSettings.CustomProvider{
		ID:       int64(p.ID),
		Name:     p.Name,
		Protocol: p.Protocol,
		Enabled:  p.Enabled,
	}
	if p.URL != "" {
		result.URL = common.StrPtr(p.URL)
	}
	if p.APIKey != "" {
		result.APIKey = common.StrPtr("***")
	}
	if p.ModelName != "" {
		result.Model = common.StrPtr(p.ModelName)
	}
	if p.AuthHeader != "" {
		result.AuthHeader = common.StrPtr(p.AuthHeader)
	}
	if len(p.ExtraBody) > 0 {
		result.ExtraBody = common.StrPtr(string(p.ExtraBody))
	}
	return result
}

// ==================== Rules Check ====================

func RulesCheck(ctx context.Context, c *app.RequestContext) {
	var req modelSettings.RulesCheckReq
	if err := c.BindAndValidate(&req); err != nil {
		common.Fail(c, common.BadRequest(err.Error()))
		return
	}

	if strings.TrimSpace(req.Path) == "" {
		common.Fail(c, common.BadRequest("文件路径不能为空"))
		return
	}

	if handlerInst.rulesChecker == nil {
		common.Fail(c, common.BadRequest("规则检查功能未配置"))
		return
	}

	result, err := handlerInst.rulesChecker.RulesCheck(ctx, req.Path)
	if err != nil {
		common.Fail(c, err)
		return
	}

	resp := &modelSettings.RulesCheckResult{
		Applicable: result.Applicable,
	}
	if result.Rule != "" {
		resp.Rule = common.StrPtr(result.Rule)
	}
	if result.Message != "" {
		resp.Message = common.StrPtr(result.Message)
	}

	common.Success(c, &modelSettings.RulesCheckResp{Result: resp})
}

// ==================== OCR Version ====================

func OCRVersion(ctx context.Context, c *app.RequestContext) {
	if handlerInst.ocrVersioner == nil {
		common.Fail(c, common.BadRequest("OCR 版本查询功能未配置"))
		return
	}

	result, err := handlerInst.ocrVersioner.OCRVersion(ctx)
	if err != nil {
		common.Fail(c, err)
		return
	}

	resp := &modelSettings.OCRVersionResp{
		Version: result.Version,
	}
	if result.Commit != "" {
		resp.Commit = common.StrPtr(result.Commit)
	}
	if result.Platform != "" {
		resp.Platform = common.StrPtr(result.Platform)
	}
	if result.BuildTime != "" {
		resp.BuildTime = common.StrPtr(result.BuildTime)
	}

	common.Success(c, resp)
}

// ==================== Viewer ====================

func ViewerStatus(ctx context.Context, c *app.RequestContext) {
	if handlerInst.viewerMgr == nil {
		common.Fail(c, common.BadRequest("Viewer 功能未配置"))
		return
	}

	result, err := handlerInst.viewerMgr.ViewerStatus(ctx)
	if err != nil {
		common.Fail(c, err)
		return
	}

	resp := &modelSettings.ViewerStatusResp{Running: result.Running}
	if result.Addr != "" {
		resp.Addr = common.StrPtr(result.Addr)
	}
	if result.Message != "" {
		resp.Message = common.StrPtr(result.Message)
	}

	common.Success(c, resp)
}

func StartViewer(ctx context.Context, c *app.RequestContext) {
	var req modelSettings.StartViewerReq
	if err := c.BindAndValidate(&req); err != nil {
		common.Fail(c, common.BadRequest(err.Error()))
		return
	}

	if handlerInst.viewerMgr == nil {
		common.Fail(c, common.BadRequest("Viewer 功能未配置"))
		return
	}

	addr := ptrx.Deref(req.Addr, "")
	result, err := handlerInst.viewerMgr.StartViewer(ctx, addr)
	if err != nil {
		common.Fail(c, err)
		return
	}

	resp := &modelSettings.StartViewerResp{Running: result.Running}
	if result.Addr != "" {
		resp.Addr = common.StrPtr(result.Addr)
	}
	if result.Message != "" {
		resp.Message = common.StrPtr(result.Message)
	}

	common.Success(c, resp)
}

func StopViewer(ctx context.Context, c *app.RequestContext) {
	if handlerInst.viewerMgr == nil {
		common.Fail(c, common.BadRequest("Viewer 功能未配置"))
		return
	}

	result, err := handlerInst.viewerMgr.StopViewer(ctx)
	if err != nil {
		common.Fail(c, err)
		return
	}

	resp := &modelSettings.ViewerStatusResp{Running: result.Running}
	if result.Addr != "" {
		resp.Addr = common.StrPtr(result.Addr)
	}
	if result.Message != "" {
		resp.Message = common.StrPtr(result.Message)
	}

	common.Success(c, resp)
}
