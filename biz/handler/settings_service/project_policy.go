package settings_service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/yi-nology/code-review-service/biz/handler/common"
	modelSettings "github.com/yi-nology/code-review-service/biz/model/settings"
	"github.com/yi-nology/code-review-service/internal/domain/projectpolicy"

	"github.com/oekit/pkg/ptrx"
	"github.com/oekit/pkg/slicex"

	"github.com/cloudwego/hertz/pkg/app"
)

func ListProjectPolicies(ctx context.Context, c *app.RequestContext) {
	items, err := handlerInst.project.List(ctx)
	if err != nil {
		common.Fail(ctx, c, err)
		return
	}
	policies := slicex.Map(items, func(item projectpolicy.ProjectPolicy) *modelSettings.ProjectPolicy {
		return dbProjectPolicyToAPI(&item)
	})
	common.Success(c, &modelSettings.ListProjectPoliciesResp{Items: policies})
}

func PutProjectPolicy(ctx context.Context, c *app.RequestContext) {
	req, ok := common.Bind[modelSettings.PutProjectPolicyReq](c)
	if !ok {
		return
	}
	dbPolicy := apiProjectPolicyToDB(req.Policy)
	// review_engine / gate_mode 未随请求上送（旧前端/客户端不感知该字段，nil）时保留
	// 库中现值，避免在设置页保存其它字段把它清空；显式上送才覆盖（空串 = 重置为跟随默认）。
	if req.Policy == nil || req.Policy.ReviewEngine == nil || req.Policy.GateMode == nil {
		if prev, err := handlerInst.project.GetByProviderRepo(ctx, dbPolicy.Provider, dbPolicy.Repo); err == nil && prev != nil {
			if req.Policy == nil || req.Policy.ReviewEngine == nil {
				dbPolicy.ReviewEngine = prev.ReviewEngine
			}
			if req.Policy == nil || req.Policy.GateMode == nil {
				dbPolicy.GateMode = prev.GateMode
			}
		}
	}
	if err := validateProjectPolicy(dbPolicy); err != nil {
		common.Fail(ctx, c, common.BadRequest(err.Error()))
		return
	}
	if err := handlerInst.project.Save(ctx, dbPolicy); err != nil {
		common.Fail(ctx, c, common.BadRequest(err.Error()))
		return
	}
	common.Success(c, &modelSettings.PutProjectPolicyResp{Policy: dbProjectPolicyToAPI(dbPolicy)})
}

func DeleteProjectPolicy(ctx context.Context, c *app.RequestContext) {
	req, ok := common.Bind[modelSettings.DeleteProjectPolicyReq](c)
	if !ok {
		return
	}
	if req.Provider == "" || req.Repo == "" {
		common.Fail(ctx, c, common.BadRequest("代码平台和仓库不能为空"))
		return
	}
	if err := handlerInst.project.Delete(ctx, req.Provider, req.Repo); err != nil {
		common.Fail(ctx, c, err)
		return
	}
	common.Success(c, &modelSettings.DeleteProjectPolicyResp{})
}

func validateProjectPolicy(p *projectpolicy.ProjectPolicy) error {
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
	switch strings.TrimSpace(p.GateMode) {
	case "", projectpolicy.GateModeStrong, projectpolicy.GateModeWeak:
	default:
		return fmt.Errorf("门禁强弱依赖模式仅支持 strong、weak，留空表示 auto（OCR 强 / argus 弱）")
	}
	return nil
}

func dbProjectPolicyToAPI(p *projectpolicy.ProjectPolicy) *modelSettings.ProjectPolicy {
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
		OcrBackgroundFile: common.StrPtr(p.OCRBackgroundFile),
		OcrAudience:       common.StrPtr(p.OCRAudience), OcrProvider: common.StrPtr(p.OCRProvider),
		OcrURL: common.StrPtr(p.OCRURL), OcrModel: common.StrPtr(p.OCRModel),
		OcrMaxGitProcs: ptrx.Of(int64(p.OCRMaxGitProcs)), OcrUseAnthropic: ptrx.Of(p.OCRUseAnthropic),
		OcrProtocol:  common.StrPtr(p.OCRProtocol),
		OcrExtraBody: common.StrPtr(p.OCRExtraBody), OcrLanguage: common.StrPtr(p.OCRLanguage),
		OcrBinary: common.StrPtr(p.OCRBinary), OcrToolsPath: common.StrPtr(p.OCRToolsPath),
		OcrTelemetryEnabled:        ptrx.Of(p.OCRTelemetryEnabled),
		OcrTelemetryExporter:       common.StrPtr(p.OCRTelemetryExporter),
		OcrTelemetryOtlpEndpoint:   common.StrPtr(p.OCRTelemetryOTLPEndpoint),
		OcrTelemetryContentLogging: ptrx.Of(p.OCRTelemetryContentLogging),
		RulePackageIds:             rulePackageIDsToAPI(p.RulePackageIDs),
		ReviewEngine:               common.StrPtr(p.ReviewEngine),
		GateMode:                   common.StrPtr(p.GateMode),
	}
}

func apiProjectPolicyToDB(p *modelSettings.ProjectPolicy) *projectpolicy.ProjectPolicy {
	db := &projectpolicy.ProjectPolicy{}
	if p == nil {
		return db
	}
	db.Provider = p.Provider
	db.Repo = p.Repo
	db.FailOn = p.FailOn
	db.ReviewEngine = strings.ToLower(strings.TrimSpace(ptrx.Deref(p.ReviewEngine, "")))
	if db.ReviewEngine != "" && db.ReviewEngine != "ocr" && db.ReviewEngine != "argus" {
		db.ReviewEngine = ""
	}
	db.GateMode = strings.ToLower(strings.TrimSpace(ptrx.Deref(p.GateMode, "")))
	if db.GateMode != "" && db.GateMode != projectpolicy.GateModeStrong && db.GateMode != projectpolicy.GateModeWeak {
		db.GateMode = "" // 非法值回退 auto（OCR 强 / argus 弱）
	}
	db.OCREnabled = p.OcrEnabled
	db.OCRConcurrency = int(ptrx.Deref(p.OcrConcurrency, 0))
	db.OCRTimeoutMinutes = int(ptrx.Deref(p.OcrTimeoutMinutes, 0))
	db.OCRMaxTools = int(ptrx.Deref(p.OcrMaxTools, 0))
	db.OCRDefaultSeverity = ptrx.Deref(p.OcrDefaultSeverity, "")
	db.OCRBackground = ptrx.Deref(p.OcrBackground, "")
	db.OCRBackgroundFile = ptrx.Deref(p.OcrBackgroundFile, "")
	db.OCRAudience = ptrx.Deref(p.OcrAudience, "")
	db.OCRProvider = ptrx.Deref(p.OcrProvider, "")
	db.OCRURL = ptrx.Deref(p.OcrURL, "")
	db.OCRModel = ptrx.Deref(p.OcrModel, "")
	db.OCRMaxGitProcs = int(ptrx.Deref(p.OcrMaxGitProcs, 0))
	db.OCRUseAnthropic = ptrx.Deref(p.OcrUseAnthropic, false)
	db.OCRProtocol = ptrx.Deref(p.OcrProtocol, "")
	db.OCRExtraBody = ptrx.Deref(p.OcrExtraBody, "")
	db.OCRLanguage = ptrx.Deref(p.OcrLanguage, "")
	db.OCRBinary = ptrx.Deref(p.OcrBinary, "")
	db.OCRToolsPath = ptrx.Deref(p.OcrToolsPath, "")
	db.OCRTelemetryEnabled = ptrx.Deref(p.OcrTelemetryEnabled, false)
	db.OCRTelemetryExporter = ptrx.Deref(p.OcrTelemetryExporter, "")
	db.OCRTelemetryOTLPEndpoint = ptrx.Deref(p.OcrTelemetryOtlpEndpoint, "")
	db.OCRTelemetryContentLogging = ptrx.Deref(p.OcrTelemetryContentLogging, false)
	if p.IsSetIgnorePaths() && p.IgnorePaths != nil && *p.IgnorePaths != "" {
		db.IgnorePaths = json.RawMessage(*p.IgnorePaths)
	}
	if p.IsSetRules() && p.Rules != nil && *p.Rules != "" {
		db.Rules = json.RawMessage(*p.Rules)
	}
	if len(p.RulePackageIds) > 0 {
		ids := make([]uint, 0, len(p.RulePackageIds))
		for _, id := range p.RulePackageIds {
			ids = append(ids, uint(id))
		}
		db.RulePackageIDs = json.RawMessage(mustMarshalJSON(ids))
	}
	return db
}
