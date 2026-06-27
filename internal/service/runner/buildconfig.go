package runner

import (
	"context"
	"strings"

	"github.com/yi-nology/code-review-service/internal/repository/postgres/models"
	"github.com/yi-nology/code-review-service/internal/service/review/domain"
)

// ToolPackageLister 是解析默认工具配置所需的最小接口。
// 满足该接口的有 repository.OCRToolPackageRepository 等。
type ToolPackageLister interface {
	List(ctx context.Context) ([]models.OCRToolPackage, error)
}

// BuildConfig 是合并多来源 OCR 配置的单一实现（runner 完整版为准）。
//
// 配置优先级（从高到低）:
//  1. overrides — 请求级 / 项目策略级覆盖（domain.OCROverrides）
//  2. settings  — 数据库全局配置（models.OCRSettings）
//  3. fallback  — YAML 文件兜底（OpenCodeReviewConfig）
//  4. 内置默认值（如 audience="human"）
//
// 包含 reasoning 解析（ResolveExtraBody）、遥测、上下文窗口、
// 以及缺失 ToolsPath/ToolsConfig 时的默认工具包回退。
//
// runner.DynamicOpenCodeReviewRunner 与 ocr.ConfigResolver 共用此实现，
// 消除此前两处 buildConfig 的重复与行为分叉。
func BuildConfig(s *models.OCRSettings, overrides domain.OCROverrides, fallback OpenCodeReviewConfig, tools ToolPackageLister) OpenCodeReviewConfig {
	cfg := OpenCodeReviewConfig{
		Binary:          firstNonBlank(s.Binary, fallback.Binary),
		Audience:        firstNonBlank(overrides.Audience, s.Audience, fallback.Audience, "human"),
		RulePath:        fallback.RulePath,
		DefaultSeverity: firstNonBlank(overrides.DefaultSeverity, s.DefaultSeverity, fallback.DefaultSeverity, string(domain.SeverityMajor)),
		URL:             firstNonBlank(overrides.URL, s.URL),
		Model:           firstNonBlank(overrides.Model, s.ModelName),
		AuthToken:       firstNonBlank(overrides.AuthToken, s.AuthToken),
		AuthHeader:      firstNonBlank(overrides.AuthHeader, s.AuthHeader),
		ExtraHeaders:    firstNonBlank(overrides.ExtraHeaders, s.ExtraHeaders),
		UseAnthropic:    overrides.UseAnthropic || s.UseAnthropic,
		ExtraBody: ResolveExtraBody(
			firstNonBlank(overrides.ExtraBody, string(s.ExtraBody)),
			firstNonBlank(s.ReasoningMode, "light"),
			s.ReasoningBudget,
			s.Provider,
		),
		Language:                firstNonBlank(overrides.Language, s.Language),
		MaxTools:                firstPositive(overrides.MaxTools, s.MaxTools),
		Background:              firstNonBlank(overrides.Background, s.Background),
		ToolsPath:               firstNonBlank(overrides.ToolsPath, s.ToolsPath),
		ToolsConfig:             string(s.ToolsConfig),
		TelemetryEnabled:        s.TelemetryEnabled,
		TelemetryExporter:       s.TelemetryExporter,
		TelemetryOTLPEndpoint:   s.TelemetryOTLPEndpoint,
		TelemetryContentLogging: s.TelemetryContentLogging,
		ContextWindow:           int(s.ContextWindow),
		MaxOutputTokens:         int(s.MaxOutputTokens),
	}
	cfg.Concurrency = firstPositive(overrides.Concurrency, s.Concurrency, fallback.Concurrency)
	cfg.MaxGitProcs = firstPositive(overrides.MaxGitProcs, s.MaxGitProcs, fallback.MaxGitProcs)
	cfg.TimeoutMinutes = firstPositive(overrides.TimeoutMinutes, s.TimeoutMinutes, fallback.TimeoutMinutes)
	if strings.TrimSpace(cfg.ToolsPath) == "" && strings.TrimSpace(cfg.ToolsConfig) == "" {
		cfg.ToolsConfig = DefaultToolConfig(tools, "diff")
	}
	return cfg
}

// DefaultToolConfig 返回首个匹配扫描模式且启用的工具包配置。
// tools 为 nil 或查询失败时返回空串。
func DefaultToolConfig(tools ToolPackageLister, mode string) string {
	if tools == nil {
		return ""
	}
	items, err := tools.List(context.TODO())
	if err != nil {
		return ""
	}
	for i := range items {
		if items[i].Enabled && (items[i].ScanType == "" || items[i].ScanType == "all" || items[i].ScanType == mode) {
			return string(items[i].Config)
		}
	}
	return ""
}
