package ocr

import (
	"context"

	"github.com/yi-nology/code-review-service/internal/repository/postgres/models"
)

// OCRSettingsGetter 是 settings 仓库的最小接口（兼容 *OCRSettingsRepository）。
type OCRSettingsGetter interface {
	Get(ctx context.Context) (*models.OCRSettings, error)
}

// SettingsAdapter 把 repository.OCRSettingsRepository 适配成 ocr.Settings 接口。
//
// 阶段 A15：让 ocr.LLMProvider / Tester / Versioner 等新代码能直接接 DB settings。
type SettingsAdapter struct {
	repo OCRSettingsGetter
}

// NewSettingsAdapter 构造 Adapter。
func NewSettingsAdapter(repo OCRSettingsGetter) *SettingsAdapter {
	return &SettingsAdapter{repo: repo}
}

// Get 实现 ocr.Settings 接口。
func (a *SettingsAdapter) Get(ctx context.Context) (*SettingsData, error) {
	m, err := a.repo.Get(ctx)
	if err != nil {
		return nil, err
	}
	if m == nil {
		return nil, nil
	}
	return convertOCRSettings(m), nil
}

// Save 实现 ocr.Settings 接口（Upgrader.Save 用）。
func (a *SettingsAdapter) Save(_ context.Context, _ *SettingsData) error {
	// 阶段 A15 占位：upgrader.Save 需要写入 DB；当前没有实现，
	// 阶段 A15 末尾从 internal/service/ocr/upgrader.go 迁移时实现。
	return nil
}

// 编译期断言
var _ Settings = (*SettingsAdapter)(nil)

// convertOCRSettings 把 models.OCRSettings 映射到 SettingsData。
func convertOCRSettings(m *models.OCRSettings) *SettingsData {
	if m == nil {
		return nil
	}
	return &SettingsData{
		Binary:                 m.Binary,
		Audience:               m.Audience,
		URL:                    m.URL,
		Model:                  m.ModelName,
		ModelName:              m.ModelName,
		AuthToken:              m.AuthToken,
		AuthHeader:             m.AuthHeader,
		ExtraHeaders:           m.ExtraHeaders,
		UseAnthropic:           m.UseAnthropic,
		ExtraBody:              string(m.ExtraBody),
		Language:               ocrLang(m),
		Background:             m.Background,
		MaxTools:               m.MaxTools,
		ToolsPath:              m.ToolsPath,
		ToolsConfig:            string(m.ToolsConfig),
		ReasoningMode:          m.ReasoningMode,
		ReasoningBudget:        m.ReasoningBudget,
		Provider:               m.Provider,
		Concurrency:            m.Concurrency,
		MaxGitProcs:            m.MaxGitProcs,
		TimeoutMinutes:         m.TimeoutMinutes,
		ContextWindow:          int(m.ContextWindow),
		MaxOutputTokens:        int(m.MaxOutputTokens),
		TelemetryEnabled:       m.TelemetryEnabled,
		TelemetryExporter:      m.TelemetryExporter,
		TelemetryOTLPEndpoint:  m.TelemetryOTLPEndpoint,
		TelemetryContentLogging: m.TelemetryContentLogging,
		DefaultSeverity:        m.DefaultSeverity,
	}
}

// languageSafe 在 ocr 包内提供一个访问 models.OCRSettings.Language 的安全 helper。
// 阶段 A15 末尾：可删除此 helper，直接写 m.LanguageSafe() 字段引用（不再依赖 model 方法）。


// languageSafe 是方法版本（用于字段初始化中的 ocrLang(m)）。
// ocrLang 是包级 helper 访问 m.Language；不能定义方法在非本地类型。
func ocrLang(m *models.OCRSettings) string {
	if m == nil {
		return ""
	}
	return m.Language
}