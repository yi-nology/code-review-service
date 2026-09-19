package settings_service

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/yi-nology/code-review-service/biz/handler/common"
	modelSettings "github.com/yi-nology/code-review-service/biz/model/settings"
	"github.com/yi-nology/code-review-service/internal/domain/llmprovider"

	"github.com/oekit/pkg/ptrx"

	"github.com/cloudwego/hertz/pkg/app"
)

func (h *SettingsHandler) listCustomProviders(ctx context.Context, c *app.RequestContext) {
	if !common.FeatureGuard(c, h.customProvider, "自定义 Provider") {
		return
	}
	providers, err := h.customProvider.List(ctx)
	if err != nil {
		common.Fail(ctx, c, err)
		return
	}
	var items []*modelSettings.CustomProvider
	for i := range providers {
		items = append(items, dbCustomProviderToAPI(&providers[i]))
	}
	common.Success(c, &modelSettings.ListCustomProvidersResp{Items: items})
}

// ListCustomProviders 包级入口（biz/router 生成代码引用）；一行委托到方法，测试可直接构造 Handler。
func ListCustomProviders(ctx context.Context, c *app.RequestContext) {
	handlerInst.listCustomProviders(ctx, c)
}

func (h *SettingsHandler) createCustomProvider(ctx context.Context, c *app.RequestContext) {
	req, ok := common.Bind[modelSettings.CreateCustomProviderReq](c)
	if !ok {
		return
	}
	if strings.TrimSpace(req.Name) == "" {
		common.Fail(ctx, c, common.BadRequest("Provider 名称不能为空"))
		return
	}
	if strings.TrimSpace(req.Protocol) == "" {
		common.Fail(ctx, c, common.BadRequest("Protocol 不能为空"))
		return
	}
	p := &llmprovider.Custom{
		Name: req.Name, Protocol: req.Protocol,
		URL: ptrx.Deref(req.URL, ""), APIKey: ptrx.Deref(req.APIKey, ""),
		ModelName: ptrx.Deref(req.Model, ""), AuthHeader: ptrx.Deref(req.AuthHeader, ""),
		Enabled: ptrx.Deref(req.Enabled, true),
	}
	if req.ExtraBody != nil && *req.ExtraBody != "" {
		p.ExtraBody = json.RawMessage(*req.ExtraBody)
	}
	if err := h.customProvider.Create(ctx, p); err != nil {
		common.Fail(ctx, c, err)
		return
	}
	common.Success(c, &modelSettings.CreateCustomProviderResp{Provider: dbCustomProviderToAPI(p)})
}

// CreateCustomProvider 包级入口（biz/router 生成代码引用）；一行委托到方法，测试可直接构造 Handler。
func CreateCustomProvider(ctx context.Context, c *app.RequestContext) {
	handlerInst.createCustomProvider(ctx, c)
}

func (h *SettingsHandler) updateCustomProvider(ctx context.Context, c *app.RequestContext) {
	req, ok := common.Bind[modelSettings.UpdateCustomProviderReq](c)
	if !ok {
		return
	}
	existing, err := h.customProvider.GetByID(ctx, common.SafeUint(req.ID))
	if err != nil || existing == nil {
		common.Fail(ctx, c, common.BadRequest("Provider 不存在"))
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
		existing.ExtraBody = json.RawMessage(*req.ExtraBody)
	}
	if req.Enabled != nil {
		existing.Enabled = *req.Enabled
	}
	if err := h.customProvider.Update(ctx, existing); err != nil {
		common.Fail(ctx, c, err)
		return
	}
	common.Success(c, &modelSettings.UpdateCustomProviderResp{Provider: dbCustomProviderToAPI(existing)})
}

// UpdateCustomProvider 包级入口（biz/router 生成代码引用）；一行委托到方法，测试可直接构造 Handler。
func UpdateCustomProvider(ctx context.Context, c *app.RequestContext) {
	handlerInst.updateCustomProvider(ctx, c)
}

func (h *SettingsHandler) deleteCustomProvider(ctx context.Context, c *app.RequestContext) {
	req, ok := common.Bind[modelSettings.DeleteCustomProviderReq](c)
	if !ok {
		return
	}
	if err := h.customProvider.Delete(ctx, common.SafeUint(req.ID)); err != nil {
		common.Fail(ctx, c, err)
		return
	}
	common.Success(c, &modelSettings.DeleteCustomProviderResp{})
}

// DeleteCustomProvider 包级入口（biz/router 生成代码引用）；一行委托到方法，测试可直接构造 Handler。
func DeleteCustomProvider(ctx context.Context, c *app.RequestContext) {
	handlerInst.deleteCustomProvider(ctx, c)
}

func dbCustomProviderToAPI(p *llmprovider.Custom) *modelSettings.CustomProvider {
	if p == nil {
		return nil
	}
	result := &modelSettings.CustomProvider{
		ID: common.SafeInt64(p.ID), Name: p.Name, Protocol: p.Protocol, Enabled: p.Enabled,
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
