package settings_service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/yi-nology/code-review-service/biz/handler/common"
	modelSettings "github.com/yi-nology/code-review-service/biz/model/settings"
	"github.com/yi-nology/code-review-service/internal/repository/postgres/models"
	"github.com/yi-nology/code-review-service/internal/service/ocr"

	"github.com/oekit/pkg/ptrx"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/oekit/logx"
	"gorm.io/datatypes"
)

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
	req, ok := common.Bind[modelSettings.UpdateOCRSettingsReq](c)
	if !ok {
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
	// OCR 启用时尝试 LLM 连通预检，但失败不阻断保存：
	// 前端已有独立 test-llm 按钮供主动验证，/health 的 llm-endpoint checker 持续监控，
	// 强制拒绝会让用户无法保存还在迭代调试的配置（用户反馈"不让保存"）。预检结果仅记日志。
	if dbSettings.Enabled {
		if tester, ok := handlerInst.llmTester.(*ocr.Tester); ok {
			testCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
			result, err := tester.TestLLMWithSettings(testCtx, dbSettings)
			cancel()
			if err != nil {
				logx.Warnf("[ocr-settings] LLM 连通测试执行失败（配置仍已保存）: %v", err)
			} else if result != nil && !result.Success {
				logx.Warnf("[ocr-settings] LLM 连通测试未通过（配置仍已保存，建议检查 LLM 配置）: %s", result.Message)
			}
		}
	}
	if err := handlerInst.ocrRepo.Save(ctx, dbSettings); err != nil {
		common.Fail(c, common.BadRequest(err.Error()))
		return
	}
	resp := dbOCRSettingsToAPI(dbSettings)
	if ptrx.Deref(resp.AuthToken, "") != "" {
		resp.AuthToken = common.StrPtr("***")
	}
	common.Success(c, &modelSettings.UpdateOCRSettingsResp{Settings: resp})
}

func TestLLM(ctx context.Context, c *app.RequestContext) {
	if !common.FeatureGuard(c, handlerInst.llmTester, "LLM 测试") {
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
		return fmt.Errorf("git 并发数需在 0 到 64 之间")
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
		DefaultSeverity:  common.StrPtr(s.DefaultSeverity),
		ToolsPath:        common.StrPtr(s.ToolsPath),
		TelemetryEnabled: ptrx.Of(s.TelemetryEnabled), TelemetryExporter: common.StrPtr(s.TelemetryExporter),
		TelemetryOtlpEndpoint: common.StrPtr(s.TelemetryOTLPEndpoint), TelemetryContentLogging: ptrx.Of(s.TelemetryContentLogging),
		ReasoningMode:   common.StrPtr(s.ReasoningMode),
		ReasoningBudget: ptrx.Of(int64(s.ReasoningBudget)),
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
	db.ReasoningMode = ptrx.Deref(s.ReasoningMode, "")
	db.ReasoningBudget = int(ptrx.Deref(s.ReasoningBudget, 0))
	if s.IsSetExtraBody() && s.ExtraBody != nil && *s.ExtraBody != "" {
		db.ExtraBody = datatypes.JSON(*s.ExtraBody)
	}
	return db
}
