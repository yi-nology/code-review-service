package settings_service

import (
	"context"

	"github.com/yi-nology/code-review-service/biz/handler/common"
	modelSettings "github.com/yi-nology/code-review-service/biz/model/settings"
	"github.com/yi-nology/code-review-service/internal/service/review"

	"github.com/cloudwego/hertz/pkg/app"
)

// CheckOCRUpdate 检查 ocr 二进制是否有新版本。
func CheckOCRUpdate(ctx context.Context, c *app.RequestContext) {
	if !common.FeatureGuard(c, handlerInst.ocrUpgrader, "OCR 升级") {
		return
	}
	info, err := handlerInst.ocrUpgrader.CheckUpdate(ctx)
	if err != nil {
		common.Fail(c, err)
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

// StartOCRUpgrade 触发 ocr 二进制在线升级（异步）。
func StartOCRUpgrade(ctx context.Context, c *app.RequestContext) {
	if !common.FeatureGuard(c, handlerInst.ocrUpgrader, "OCR 升级") {
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
	status, err := handlerInst.ocrUpgrader.Upgrade(ctx, version)
	if err != nil {
		if status != nil {
			common.Fail(c, common.BizErr(409, err.Error()))
			return
		}
		common.Fail(c, err)
		return
	}
	common.Success(c, upgradeStatusToResp(status))
}

// OCRUpgradeStatus 查询当前升级任务的状态。
func OCRUpgradeStatus(ctx context.Context, c *app.RequestContext) {
	if !common.FeatureGuard(c, handlerInst.ocrUpgrader, "OCR 升级") {
		return
	}
	status, err := handlerInst.ocrUpgrader.UpgradeStatus(ctx)
	if err != nil {
		common.Fail(c, err)
		return
	}
	common.Success(c, upgradeStatusToResp(status))
}

func upgradeStatusToResp(s *review.OCRUpgradeStatus) *modelSettings.OCRUpgradeStatusResp {
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
