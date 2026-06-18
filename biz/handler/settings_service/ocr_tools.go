package settings_service

import (
	"context"

	"github.com/yi-nology/code-review-service/biz/handler/common"
	modelSettings "github.com/yi-nology/code-review-service/biz/model/settings"

	"github.com/oekit/pkg/ptrx"

	"github.com/cloudwego/hertz/pkg/app"
)

func RulesCheck(ctx context.Context, c *app.RequestContext) {
	var req modelSettings.RulesCheckReq
	if err := c.BindAndValidate(&req); err != nil {
		common.Fail(c, common.BadRequest(err.Error()))
		return
	}
	if req.Path == "" {
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
	resp := &modelSettings.RulesCheckResult{Applicable: result.Applicable}
	if result.Rule != "" {
		resp.Rule = common.StrPtr(result.Rule)
	}
	if result.Message != "" {
		resp.Message = common.StrPtr(result.Message)
	}
	common.Success(c, &modelSettings.RulesCheckResp{Result: resp})
}

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
	resp := &modelSettings.OCRVersionResp{Version: result.Version}
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
		item := &modelSettings.LLMProvider{Name: p.Name, Protocol: p.Protocol}
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
