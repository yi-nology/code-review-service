package settings_service

import (
	"context"

	"github.com/yi-nology/code-review-service/biz/handler/common"
	modelSettings "github.com/yi-nology/code-review-service/biz/model/settings"
	"github.com/yi-nology/code-review-service/internal/repository/postgres/models"

	"github.com/oekit/pkg/ptrx"
	"github.com/oekit/pkg/slicex"

	"github.com/cloudwego/hertz/pkg/app"
)

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
