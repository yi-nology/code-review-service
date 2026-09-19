package settings_service

import (
	"context"

	"github.com/yi-nology/code-review-service/biz/handler/common"
	modelSettings "github.com/yi-nology/code-review-service/biz/model/settings"
	"github.com/yi-nology/code-review-service/internal/service/review/domain"

	"github.com/cloudwego/hertz/pkg/app"
)

// CheckOCRUpdate 检查 ocr 二进制是否有新版本。
func (h *SettingsHandler) checkOCRUpdate(ctx context.Context, c *app.RequestContext) {
	if !common.FeatureGuard(c, h.ocr.Upgrader(), "OCR 升级") {
		return
	}
	info, err := h.ocr.Upgrader().CheckUpdate(ctx)
	if err != nil {
		common.Fail(ctx, c, err)
		return
	}
	resp := &modelSettings.OCRUpdateInfoResp{
		CurrentVersion:  info.CurrentVersion,
		LatestVersion:   info.LatestVersion,
		UpdateAvailable: info.UpdateAvailable,
	}
	if info.ReleaseNotes != "" {
		resp.ReleaseNotes = common.StrPtr(info.ReleaseNotes)
	}
	if info.ReleaseURL != "" {
		resp.ReleaseURL = common.StrPtr(info.ReleaseURL)
	}
	common.Success(c, resp)
}

// CheckOCRUpdate 包级入口（biz/router 生成代码引用）；一行委托到方法，测试可直接构造 Handler。
func CheckOCRUpdate(ctx context.Context, c *app.RequestContext) { handlerInst.checkOCRUpdate(ctx, c) }

// StartOCRUpgrade 触发 ocr 二进制在线升级（异步）。
func (h *SettingsHandler) startOCRUpgrade(ctx context.Context, c *app.RequestContext) {
	if !common.FeatureGuard(c, h.ocr.Upgrader(), "OCR 升级") {
		return
	}
	req, ok := common.Bind[modelSettings.StartOCRUpgradeReq](c)
	if !ok {
		return
	}
	version := ""
	if req.Version != nil {
		version = *req.Version
	}
	status, err := h.ocr.Upgrader().Upgrade(ctx, version)
	if err != nil {
		if status != nil {
			common.Fail(ctx, c, common.BizErr(409, err.Error()))
			return
		}
		common.Fail(ctx, c, err)
		return
	}
	common.Success(c, upgradeStatusToResp(status))
}

// StartOCRUpgrade 包级入口（biz/router 生成代码引用）；一行委托到方法，测试可直接构造 Handler。
func StartOCRUpgrade(ctx context.Context, c *app.RequestContext) { handlerInst.startOCRUpgrade(ctx, c) }

// OCRUpgradeStatus 查询当前升级任务的状态。
func (h *SettingsHandler) oCRUpgradeStatus(ctx context.Context, c *app.RequestContext) {
	if !common.FeatureGuard(c, h.ocr.Upgrader(), "OCR 升级") {
		return
	}
	status, err := h.ocr.Upgrader().UpgradeStatus(ctx)
	if err != nil {
		common.Fail(ctx, c, err)
		return
	}
	common.Success(c, upgradeStatusToResp(status))
}

// OCRUpgradeStatus 包级入口（biz/router 生成代码引用）；一行委托到方法，测试可直接构造 Handler。
func OCRUpgradeStatus(ctx context.Context, c *app.RequestContext) {
	handlerInst.oCRUpgradeStatus(ctx, c)
}

func upgradeStatusToResp(s *domain.OCRUpgradeStatus) *modelSettings.OCRUpgradeStatusResp {
	if s == nil {
		return &modelSettings.OCRUpgradeStatusResp{State: "idle"}
	}
	resp := &modelSettings.OCRUpgradeStatusResp{State: string(s.State)}
	if s.Message != "" {
		resp.Message = common.StrPtr(s.Message)
	}
	if s.Version != "" {
		resp.Version = common.StrPtr(s.Version)
	}
	if s.Progress > 0 {
		p := int32(s.Progress)
		resp.Progress = &p
	}
	return resp
}
