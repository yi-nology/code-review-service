package ocr

import (
	"context"

	"github.com/yi-nology/code-review-service/internal/domain/ocrsettings"
)

// OCRSettingsGetter 是 settings 仓库的最小接口（兼容 *OCRSettingsRepository）。
// #6 迁移：已改为返回 domain 类型，ocr 包不再依赖 postgres/models。
type OCRSettingsGetter interface {
	Get(ctx context.Context) (*ocrsettings.Settings, error)
}

// SettingsAdapter 把 repository.OCRSettingsRepository 适配成 ocr.Settings 接口。
//
// 让 ocr 侧的 Provider / Tester / Versioner 等代码能直接接 DB settings。
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
	// 占位：upgrader.Save 需要写入 DB；当前没有实现，
	// 待从 internal/service/ocr/upgrader.go 迁移时实现。
	return nil
}

// 编译期断言
var _ Settings = (*SettingsAdapter)(nil)

// convertOCRSettings 把 domain ocrsettings.Settings 映射到 SettingsData（#6：不再依赖 models）。
func convertOCRSettings(m *ocrsettings.Settings) *SettingsData {
	if m == nil {
		return nil
	}
	return &SettingsData{
		Binary:                  m.Binary,
		Audience:                m.Audience,
		URL:                     m.URL,
		Model:                   m.ModelName,
		ModelName:               m.ModelName,
		AuthToken:               m.AuthToken,
		AuthHeader:              m.AuthHeader,
		ExtraHeaders:            m.ExtraHeaders,
		UseAnthropic:            m.UseAnthropic,
		Protocol:                m.Protocol,
		ExtraBody:               string(m.ExtraBody),
		Language:                m.Language,
		Background:              m.Background,
		BackgroundFile:          m.BackgroundFile,
		MaxTools:                m.MaxTools,
		ToolsPath:               m.ToolsPath,
		ToolsConfig:             string(m.ToolsConfig),
		McpServers:              string(m.McpServers),
		ReasoningMode:           m.ReasoningMode,
		ReasoningBudget:         m.ReasoningBudget,
		Provider:                m.Provider,
		Concurrency:             m.Concurrency,
		MaxGitProcs:             m.MaxGitProcs,
		TimeoutMinutes:          m.TimeoutMinutes,
		ContextWindow:           int(m.ContextWindow),
		MaxOutputTokens:         int(m.MaxOutputTokens),
		TelemetryEnabled:        m.TelemetryEnabled,
		TelemetryExporter:       m.TelemetryExporter,
		TelemetryOTLPEndpoint:   m.TelemetryOTLPEndpoint,
		TelemetryContentLogging: m.TelemetryContentLogging,
		DefaultSeverity:         m.DefaultSeverity,
		GitHubMirror:            m.GitHubMirror,
	}
}
