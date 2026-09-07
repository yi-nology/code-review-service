package settings_service

import (
	"context"
	"fmt"

	"github.com/yi-nology/code-review-service/biz/handler/common"
	modelSettings "github.com/yi-nology/code-review-service/biz/model/settings"
	"github.com/yi-nology/code-review-service/internal/domain/platform"

	"github.com/oekit/logx"
	"github.com/oekit/pkg/ptrx"
	"github.com/oekit/pkg/slicex"

	"github.com/cloudwego/hertz/pkg/app"
)

func ListPlatforms(ctx context.Context, c *app.RequestContext) {
	items, err := handlerInst.platform.List(ctx)
	if err != nil {
		common.Fail(ctx, c, err)
		return
	}
	platforms := slicex.Map(items, func(item platform.Platform) *modelSettings.GitPlatformInfo {
		return &modelSettings.GitPlatformInfo{
			Provider: item.Provider, BaseURL: item.BaseURL,
			Token: common.MaskToken(item.Token), Enabled: item.Enabled,
			WebhookSecret:  common.MaskWebhookSecret(item.WebhookSecret),
			InstallationID: &item.InstallationID,
			SkipTLS:        &item.SkipTLS,
		}
	})
	common.Success(c, &modelSettings.ListPlatformsResp{Items: platforms})
}

func PutPlatform(ctx context.Context, c *app.RequestContext) {
	req, ok := common.Bind[modelSettings.PutPlatformReq](c)
	if !ok {
		return
	}
	if req.Provider == "" {
		common.Fail(ctx, c, common.BadRequest("代码平台不能为空"))
		return
	}
	token := ptrx.Deref(req.Token, "")
	if token == "" {
		installationIDForLookup := ptrx.Deref(req.InstallationID, "")
		if installationIDForLookup == "" {
			installationIDForLookup = "default"
		}
		existing, err := handlerInst.platform.Get(ctx, req.Provider, installationIDForLookup)
		if err == nil && existing != nil {
			token = existing.Token
		}
	}
	if token == "" {
		common.Fail(ctx, c, common.BadRequest("Token 不能为空"))
		return
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	installationID := ptrx.Deref(req.InstallationID, "")
	if installationID == "" {
		installationID = "default"
	}
	p := &platform.Platform{
		Provider: req.Provider, BaseURL: ptrx.Deref(req.BaseURL, ""),
		Token: token, WebhookSecret: ptrx.Deref(req.WebhookSecret, ""), Enabled: enabled,
		InstallationID: installationID, SkipTLS: ptrx.Deref(req.SkipTLS, false),
	}
	if err := handlerInst.platform.Save(ctx, p); err != nil {
		common.Fail(ctx, c, common.BizErr(500, "保存平台配置失败"))
		return
	}
	if handlerInst.gitMgr != nil {
		if enabled {
			if err := handlerInst.gitMgr.AddOrUpdateFromDB(p); err != nil {
				common.Fail(ctx, c, fmt.Errorf("注册平台失败: %w", err))
				return
			}
		} else {
			handlerInst.gitMgr.Remove(p.Provider)
		}
	}
	common.Success(c, &modelSettings.PutPlatformResp{
		Platform: &modelSettings.GitPlatformInfo{
			Provider: p.Provider, BaseURL: p.BaseURL,
			Token: common.MaskToken(p.Token), Enabled: p.Enabled,
			WebhookSecret:  common.MaskWebhookSecret(p.WebhookSecret),
			InstallationID: &p.InstallationID,
			SkipTLS:        &p.SkipTLS,
		},
	})
}

func DeletePlatform(ctx context.Context, c *app.RequestContext) {
	req, ok := common.Bind[modelSettings.DeletePlatformReq](c)
	if !ok {
		return
	}
	if req.Provider == "" {
		common.Fail(ctx, c, common.BadRequest("代码平台不能为空"))
		return
	}
	installationID := ptrx.Deref(req.InstallationID, "")
	if installationID == "" {
		installationID = "default"
	}
	// 按 (provider, installation_id) 精确删除，避免误删同 provider 其它 installation。
	if err := handlerInst.platform.Delete(ctx, req.Provider, installationID); err != nil {
		common.Fail(ctx, c, err)
		return
	}
	// 运行态 provider map 是 per-provider 的（非 per-installation），删除一个 installation
	// 不应直接清掉整个 provider 运行态。仅当该 provider 已无任何记录时才 Remove，
	// 否则会误伤同 provider 的其它 installation（它们在重启/重载前仍需运行态可用）。
	if handlerInst.gitMgr != nil {
		remaining, err := handlerInst.platform.List(ctx)
		if err == nil {
			hasOther := false
			for _, p := range remaining {
				if p.Provider == req.Provider {
					hasOther = true
					break
				}
			}
			if !hasOther {
				handlerInst.gitMgr.Remove(req.Provider)
			}
		} else {
			// List 失败时保守处理：不擅自移除运行态，避免误删。
			logx.Warnf("[platform] delete %s/%s 后 List 失败，跳过运行态清理: %v", req.Provider, installationID, err)
		}
	}
	common.Success(c, &modelSettings.DeletePlatformResp{})
}

func TestPlatform(ctx context.Context, c *app.RequestContext) {
	req, ok := common.Bind[modelSettings.TestPlatformReq](c)
	if !ok {
		return
	}
	if req.Provider == "" || req.Token == "" {
		common.Fail(ctx, c, common.BadRequest("平台和 Token 不能为空"))
		return
	}
	result, err := handlerInst.gitMgr.TestWithToken(ctx, req.Provider, ptrx.Deref(req.BaseURL, ""), req.Token, ptrx.Deref(req.SkipTLS, false))
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
