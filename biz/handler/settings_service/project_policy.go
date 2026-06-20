package settings_service

import (
	"context"
	"fmt"
	"strings"

	"github.com/yi-nology/code-review-service/biz/handler/common"
	modelSettings "github.com/yi-nology/code-review-service/biz/model/settings"
	"github.com/yi-nology/code-review-service/internal/repository/postgres/models"

	"github.com/oekit/pkg/ptrx"
	"github.com/oekit/pkg/slicex"

	"github.com/cloudwego/hertz/pkg/app"
	"gorm.io/datatypes"
)

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
	req, ok := common.Bind[modelSettings.PutProjectPolicyReq](c)
	if !ok { return }
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

func DeleteProjectPolicy(ctx context.Context, c *app.RequestContext) {
	req, ok := common.Bind[modelSettings.DeleteProjectPolicyReq](c)
	if !ok { return }
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
		OcrTelemetryEnabled:        ptrx.Of(p.OCRTelemetryEnabled),
		OcrTelemetryExporter:       common.StrPtr(p.OCRTelemetryExporter),
		OcrTelemetryOtlpEndpoint:   common.StrPtr(p.OCRTelemetryOTLPEndpoint),
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
