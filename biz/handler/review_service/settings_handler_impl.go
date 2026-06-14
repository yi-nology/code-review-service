package review_service

import (
	"context"
	"encoding/json"
	"fmt"

	modelSettings "github.com/yi-nology/code-review-service/biz/model/settings"
	"github.com/yi-nology/code-review-service/internal/repository/postgres/models"

	"github.com/oekit/pkg/convert"
	"github.com/oekit/pkg/mathx"
	"github.com/oekit/pkg/ptrx"
	"github.com/oekit/pkg/slicex"
	"github.com/oekit/pkg/stringx"

	hertzApp "github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	provider_sdk "github.com/yi-nology/git-platform-sdk/provider"
	"gorm.io/datatypes"
)

func (h *ReviewHandler) GetOCRSettings(ctx context.Context, c *hertzApp.RequestContext) {
	settings, err := h.ocrRepo.Get(ctx)
	if err != nil {
		fail(c, err)
		return
	}

	resp := dbOCRSettingsToAPI(settings)
	if ptrx.Deref(resp.AuthToken, "") != "" {
		resp.AuthToken = strPtr("***")
	}

	c.JSON(consts.StatusOK, &modelSettings.GetOCRSettingsResp{
		Settings: resp,
	})
}

func (h *ReviewHandler) UpdateOCRSettings(ctx context.Context, c *hertzApp.RequestContext) {
	var req modelSettings.UpdateOCRSettingsReq
	if err := c.BindAndValidate(&req); err != nil {
		fail(c, badRequest(err.Error()))
		return
	}

	dbSettings := apiOCRSettingsToDB(req.Settings)

	if dbSettings.AuthToken == "" || dbSettings.AuthToken == "***" {
		existing, err := h.ocrRepo.Get(ctx)
		if err == nil && existing != nil {
			dbSettings.AuthToken = existing.AuthToken
		}
	}

	if err := h.ocrRepo.Save(ctx, dbSettings); err != nil {
		fail(c, badRequest(err.Error()))
		return
	}

	resp := dbOCRSettingsToAPI(dbSettings)
	if ptrx.Deref(resp.AuthToken, "") != "" {
		resp.AuthToken = strPtr("***")
	}

	c.JSON(consts.StatusOK, &modelSettings.UpdateOCRSettingsResp{
		Settings: resp,
	})
}

func (h *ReviewHandler) ListProjectPolicies(ctx context.Context, c *hertzApp.RequestContext) {
	items, err := h.project.List(ctx)
	if err != nil {
		fail(c, err)
		return
	}

	policies := slicex.Map(items, func(item models.ProjectPolicy) *modelSettings.ProjectPolicy {
		return dbProjectPolicyToAPI(&item)
	})

	c.JSON(consts.StatusOK, &modelSettings.ListProjectPoliciesResp{
		Items: policies,
	})
}

func (h *ReviewHandler) PutProjectPolicy(ctx context.Context, c *hertzApp.RequestContext) {
	var req modelSettings.PutProjectPolicyReq
	if err := c.BindAndValidate(&req); err != nil {
		fail(c, badRequest(err.Error()))
		return
	}

	dbPolicy := apiProjectPolicyToDB(req.Policy)
	if err := h.project.Save(ctx, dbPolicy); err != nil {
		fail(c, badRequest(err.Error()))
		return
	}

	c.JSON(consts.StatusOK, &modelSettings.PutProjectPolicyResp{
		Policy: dbProjectPolicyToAPI(dbPolicy),
	})
}

func (h *ReviewHandler) DeleteProjectPolicy(ctx context.Context, c *hertzApp.RequestContext) {
	var req modelSettings.DeleteProjectPolicyReq
	if err := c.BindAndValidate(&req); err != nil {
		fail(c, badRequest(err.Error()))
		return
	}
	if req.Provider == "" || req.Repo == "" {
		fail(c, badRequest("代码平台和仓库不能为空"))
		return
	}
	if err := h.project.Delete(ctx, req.Provider, req.Repo); err != nil {
		fail(c, err)
		return
	}
	c.JSON(consts.StatusOK, &modelSettings.DeleteProjectPolicyResp{})
}

func (h *ReviewHandler) ListPlatforms(ctx context.Context, c *hertzApp.RequestContext) {
	items, err := h.platform.List(ctx)
	if err != nil {
		fail(c, err)
		return
	}

	platforms := slicex.Map(items, func(item models.GitPlatform) *modelSettings.GitPlatformInfo {
		return &modelSettings.GitPlatformInfo{
			Provider:      item.Provider,
			BaseURL:       item.BaseURL,
			Token:         maskToken(item.Token),
			Enabled:       item.Enabled,
			WebhookSecret: maskWebhookSecret(item.WebhookSecret),
		}
	})

	c.JSON(consts.StatusOK, &modelSettings.ListPlatformsResp{
		Items: platforms,
	})
}

func (h *ReviewHandler) PutPlatform(ctx context.Context, c *hertzApp.RequestContext) {
	var req modelSettings.PutPlatformReq
	if err := c.BindAndValidate(&req); err != nil {
		fail(c, badRequest(err.Error()))
		return
	}
	if req.Provider == "" {
		fail(c, badRequest("代码平台不能为空"))
		return
	}

	token := ptrx.Deref(req.Token, "")
	if token == "" {
		existing, err := h.platform.Get(ctx, req.Provider)
		if err == nil && existing != nil {
			token = existing.Token
		}
	}
	if token == "" {
		fail(c, badRequest("Token 不能为空"))
		return
	}

	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	p := &models.GitPlatform{
		Provider:      req.Provider,
		BaseURL:       ptrx.Deref(req.BaseURL, ""),
		Token:         token,
		WebhookSecret: ptrx.Deref(req.WebhookSecret, ""),
		Enabled:       enabled,
	}
	if err := h.platform.Save(ctx, p); err != nil {
		fail(c, bizErr(500, "保存平台配置失败"))
		return
	}

	if h.gitMgr != nil {
		if enabled {
			_ = h.gitMgr.AddOrUpdateFromDB(p)
		} else {
			h.gitMgr.Remove(p.Provider)
		}
	}

	c.JSON(consts.StatusOK, &modelSettings.PutPlatformResp{
		Platform: &modelSettings.GitPlatformInfo{
			Provider:      p.Provider,
			BaseURL:       p.BaseURL,
			Token:         maskToken(p.Token),
			Enabled:       p.Enabled,
			WebhookSecret: maskWebhookSecret(p.WebhookSecret),
		},
	})
}

func (h *ReviewHandler) DeletePlatform(ctx context.Context, c *hertzApp.RequestContext) {
	var req modelSettings.DeletePlatformReq
	if err := c.BindAndValidate(&req); err != nil {
		fail(c, badRequest(err.Error()))
		return
	}
	if req.Provider == "" {
		fail(c, badRequest("代码平台不能为空"))
		return
	}
	if err := h.platform.Delete(ctx, req.Provider); err != nil {
		fail(c, err)
		return
	}
	if h.gitMgr != nil {
		h.gitMgr.Remove(req.Provider)
	}
	c.JSON(consts.StatusOK, &modelSettings.DeletePlatformResp{})
}

func (h *ReviewHandler) TestPlatform(ctx context.Context, c *hertzApp.RequestContext) {
	var req modelSettings.TestPlatformReq
	if err := c.BindAndValidate(&req); err != nil {
		fail(c, badRequest(err.Error()))
		return
	}
	if req.Provider == "" || req.Token == "" {
		fail(c, badRequest("平台和 Token 不能为空"))
		return
	}

	result, err := h.gitMgr.TestWithToken(ctx, req.Provider, ptrx.Deref(req.BaseURL, ""), req.Token)
	if err != nil {
		c.JSON(consts.StatusOK, &modelSettings.TestPlatformResp{
			Result: &modelSettings.GitTestResult{
				Connected: false,
				Platform:  req.Provider,
				Message:   strPtr(err.Error()),
			},
		})
		return
	}
	c.JSON(consts.StatusOK, &modelSettings.TestPlatformResp{
		Result: &modelSettings.GitTestResult{
			Connected: result.Connected,
			Platform:  result.Platform,
			UserName:  strPtr(result.UserName),
			Message:   strPtr(result.Message),
		},
	})
}

func dbOCRSettingsToAPI(s *models.OCRSettings) *modelSettings.OCRSettings {
	var extraBody string
	if len(s.ExtraBody) > 0 {
		extraBody = string(s.ExtraBody)
	}
	return &modelSettings.OCRSettings{
		Enabled:        s.Enabled,
		Provider:       strPtr(s.Provider),
		Plan:           strPtr(s.Plan),
		URL:            strPtr(s.URL),
		AuthToken:      strPtr(s.AuthToken),
		AuthHeader:     strPtr(s.AuthHeader),
		Model:          strPtr(s.ModelName),
		UseAnthropic:   ptrx.Of(s.UseAnthropic),
		ExtraBody:      strPtr(extraBody),
		Language:       strPtr(s.Language),
		Concurrency:    ptrx.Of(int64(s.Concurrency)),
		MaxGitProcs:    ptrx.Of(int64(s.MaxGitProcs)),
		TimeoutMinutes: ptrx.Of(int64(s.TimeoutMinutes)),
		MaxTools:       ptrx.Of(int64(s.MaxTools)),
		Background:     strPtr(s.Background),
	}
}

func apiOCRSettingsToDB(s *modelSettings.OCRSettings) *models.OCRSettings {
	db := &models.OCRSettings{}
	if s == nil {
		return db
	}
	db.Enabled = s.Enabled
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
	var ignorePaths string
	if len(p.IgnorePaths) > 0 {
		ignorePaths = string(p.IgnorePaths)
	}
	var rules string
	if len(p.Rules) > 0 {
		rules = string(p.Rules)
	}
	return &modelSettings.ProjectPolicy{
		Provider:    p.Provider,
		Repo:        p.Repo,
		FailOn:      p.FailOn,
		IgnorePaths: strPtr(ignorePaths),
		Rules:       strPtr(rules),
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

func (h *ReviewHandler) GetGlobalRules(ctx context.Context, c *hertzApp.RequestContext) {
	rules, err := h.globalRules.Get(ctx)
	if err != nil {
		fail(c, err)
		return
	}

	resp := dbGlobalRulesToAPI(rules)
	c.JSON(consts.StatusOK, &modelSettings.GetGlobalRulesResp{
		Rules: resp,
	})
}

func (h *ReviewHandler) UpdateGlobalRules(ctx context.Context, c *hertzApp.RequestContext) {
	var req modelSettings.UpdateGlobalRulesReq
	if err := c.BindAndValidate(&req); err != nil {
		fail(c, badRequest(err.Error()))
		return
	}

	dbRules := apiGlobalRulesToDB(req.Rules)
	if err := h.globalRules.Save(ctx, dbRules); err != nil {
		fail(c, bizErr(500, "保存全局规则失败"))
		return
	}

	c.JSON(consts.StatusOK, &modelSettings.UpdateGlobalRulesResp{
		Rules: dbGlobalRulesToAPI(dbRules),
	})
}

func (h *ReviewHandler) TestLLM(ctx context.Context, c *hertzApp.RequestContext) {
	if h.llmTester == nil {
		fail(c, badRequest("LLM 测试功能未配置"))
		return
	}

	result, err := h.llmTester.TestLLM(ctx)
	if err != nil {
		c.JSON(consts.StatusOK, &modelSettings.TestLLMResp{
			Result: &modelSettings.TestLLMResult{
				Success: false,
				Message: strPtr(err.Error()),
			},
		})
		return
	}

	c.JSON(consts.StatusOK, &modelSettings.TestLLMResp{
		Result: &modelSettings.TestLLMResult{
			Success: result.Success,
			Message: strPtr(result.Message),
		},
	})
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
				entries = append(entries, &modelSettings.ReviewRule{
					Path: r.Path,
					Rule: r.Rule,
				})
			}
		}
	}
	return &modelSettings.GlobalRules{Rules: entries}
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

func (h *ReviewHandler) ListProviderRepos(ctx context.Context, c *hertzApp.RequestContext) {
	provider := c.Param("provider")
	if provider == "" {
		fail(c, badRequest("代码平台不能为空"))
		return
	}

	if h.gitMgr == nil || !h.gitMgr.HasProvider(provider) {
		fail(c, badRequest(fmt.Sprintf("未配置的代码平台: %s", provider)))
		return
	}

	page := mathx.Clamp(convert.Str2Int(c.Query("page"), 1), 1, 999999)
	perPage := mathx.Clamp(convert.Str2Int(c.Query("per_page"), 30), 1, 100)

	repos, err := h.gitMgr.ListRepos(ctx, provider, provider_sdk.ListRepoOptions{
		Page:    page,
		PerPage: perPage,
	})
	if err != nil {
		fail(c, bizErr(500, fmt.Sprintf("获取仓库列表失败: %v", err)))
		return
	}

	items := make([]map[string]any, 0, len(repos))
	for _, r := range repos {
		items = append(items, map[string]any{
			"id":             r.ID,
			"full_name":      r.FullName,
			"name":           r.Name,
			"owner":          r.Owner,
			"description":    r.Description,
			"default_branch": r.DefaultBranch,
			"private":        r.Private,
		})
	}

	c.JSON(consts.StatusOK, map[string]any{
		"items": items,
	})
}

func (h *ReviewHandler) ListProviderBranches(ctx context.Context, c *hertzApp.RequestContext) {
	provider := c.Param("provider")
	owner := c.Param("owner")
	repo := c.Param("repo")

	if provider == "" || owner == "" || repo == "" {
		fail(c, badRequest("参数不完整"))
		return
	}

	if h.gitMgr == nil || !h.gitMgr.HasProvider(provider) {
		fail(c, badRequest(fmt.Sprintf("未配置的代码平台: %s", provider)))
		return
	}

	branches, err := h.gitMgr.ListBranches(ctx, provider, owner, repo)
	if err != nil {
		fail(c, bizErr(500, fmt.Sprintf("获取分支列表失败: %v", err)))
		return
	}

	items := make([]map[string]any, 0, len(branches))
	for _, b := range branches {
		items = append(items, map[string]any{
			"name": b.Name,
		})
	}

	c.JSON(consts.StatusOK, map[string]any{
		"items": items,
	})
}

func (h *ReviewHandler) ListProviderPulls(ctx context.Context, c *hertzApp.RequestContext) {
	provider := c.Param("provider")
	owner := c.Param("owner")
	repo := c.Param("repo")

	if provider == "" || owner == "" || repo == "" {
		fail(c, badRequest("参数不完整"))
		return
	}

	if h.gitMgr == nil || !h.gitMgr.HasProvider(provider) {
		fail(c, badRequest(fmt.Sprintf("未配置的代码平台: %s", provider)))
		return
	}

	page := mathx.Clamp(convert.Str2Int(c.Query("page"), 1), 1, 999999)
	perPage := mathx.Clamp(convert.Str2Int(c.Query("per_page"), 30), 1, 100)
	state := stringx.TakeOne(c.Query("state"), "opened")

	crs, _, err := h.gitMgr.ListCRs(ctx, provider, provider_sdk.ListCROptions{
		Owner:   owner,
		Repo:    repo,
		State:   provider_sdk.CRState(state),
		Page:    page,
		PerPage: perPage,
	})
	if err != nil {
		fail(c, bizErr(500, fmt.Sprintf("获取 PR/MR 列表失败: %v", err)))
		return
	}

	items := make([]map[string]any, 0, len(crs))
	for _, cr := range crs {
		item := map[string]any{
			"number":        cr.Number,
			"title":         cr.Title,
			"state":         string(cr.State),
			"source_branch": cr.SourceBranch,
			"target_branch": cr.TargetBranch,
			"head_sha":      cr.HeadSHA,
			"base_sha":      cr.BaseSHA,
			"web_url":       cr.WebURL,
			"created_at":    cr.CreatedAt,
			"updated_at":    cr.UpdatedAt,
		}
		if cr.Author != nil {
			item["author"] = map[string]any{
				"username":   cr.Author.Username,
				"name":       cr.Author.Name,
				"avatar_url": cr.Author.AvatarURL,
			}
		}
		items = append(items, item)
	}

	c.JSON(consts.StatusOK, map[string]any{
		"items": items,
	})
}

func (h *ReviewHandler) GetBranchSHA(ctx context.Context, c *hertzApp.RequestContext) {
	provider := c.Param("provider")
	owner := c.Param("owner")
	repo := c.Param("repo")
	branch := c.Query("branch")

	if provider == "" || owner == "" || repo == "" || branch == "" {
		fail(c, badRequest("参数不完整"))
		return
	}

	if h.gitMgr == nil || !h.gitMgr.HasProvider(provider) {
		fail(c, badRequest(fmt.Sprintf("未配置的代码平台: %s", provider)))
		return
	}

	sha, err := h.gitMgr.GetBranchSHA(ctx, provider, owner, repo, branch)
	if err != nil {
		fail(c, bizErr(500, fmt.Sprintf("获取分支 SHA 失败: %v", err)))
		return
	}

	c.JSON(consts.StatusOK, map[string]any{
		"sha": sha,
	})
}

func (h *ReviewHandler) ListReviewerAgents(ctx context.Context, c *hertzApp.RequestContext) {
	agents, err := h.agentRepo.List(ctx)
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(consts.StatusOK, map[string]any{
		"agents": agents,
	})
}

func (h *ReviewHandler) CreateReviewerAgent(ctx context.Context, c *hertzApp.RequestContext) {
	var req struct {
		Name        string   `json:"name" binding:"required"`
		DisplayName string   `json:"display_name" binding:"required"`
		Description string   `json:"description"`
		Background  string   `json:"background" binding:"required"`
		Enabled     *bool    `json:"enabled"`
		Priority    *int64   `json:"priority"`
		Tags        []string `json:"tags"`
	}
	if err := c.BindAndValidate(&req); err != nil {
		fail(c, badRequest(err.Error()))
		return
	}

	agent := &models.ReviewerAgent{
		Name:        req.Name,
		DisplayName: req.DisplayName,
		Description: req.Description,
		Background:  req.Background,
		Enabled:     true,
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

	if err := h.agentRepo.Create(ctx, agent); err != nil {
		fail(c, err)
		return
	}

	c.JSON(consts.StatusOK, map[string]any{
		"agent": agent,
	})
}

func (h *ReviewHandler) UpdateReviewerAgent(ctx context.Context, c *hertzApp.RequestContext) {
	idStr := c.Param("id")
	id, err := parseUintID(idStr)
	if err != nil {
		fail(c, badRequest("invalid agent id"))
		return
	}

	existing, err := h.agentRepo.GetByID(ctx, id)
	if err != nil {
		fail(c, notFound("agent not found"))
		return
	}

	var req struct {
		DisplayName *string  `json:"display_name"`
		Description *string  `json:"description"`
		Background  *string  `json:"background"`
		Enabled     *bool    `json:"enabled"`
		Priority    *int64   `json:"priority"`
		Tags        []string `json:"tags"`
	}
	if err := c.BindAndValidate(&req); err != nil {
		fail(c, badRequest(err.Error()))
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

	if err := h.agentRepo.Update(ctx, existing); err != nil {
		fail(c, err)
		return
	}

	c.JSON(consts.StatusOK, map[string]any{
		"agent": existing,
	})
}

func (h *ReviewHandler) DeleteReviewerAgent(ctx context.Context, c *hertzApp.RequestContext) {
	idStr := c.Param("id")
	id, err := parseUintID(idStr)
	if err != nil {
		fail(c, badRequest("invalid agent id"))
		return
	}

	if err := h.agentRepo.Delete(ctx, id); err != nil {
		fail(c, err)
		return
	}

	c.JSON(consts.StatusOK, map[string]any{})
}

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
