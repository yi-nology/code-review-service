package ocr

import (
	"context"
	"fmt"
)

// Settings 是「读 OCR 配置」的唯一接口。
//
// 阶段 A 修复：原 5 个等价接口（ocr/settingsSource / runner/ocrSettingsSource /
// llm/settingsSource / ocr/upgrader.settingsStore / runner/toolPackageSource）
// 全部统一为这一个。
//
// 满足该接口的有：
//   - repository.OCRSettingsRepository（包装后的 adapter）
//   - *SettingsSnapshot（用于测试与 fallback）
//   - ocr.Client 内部 sub-settings（如 GetOverrideByProject）
type Settings interface {
	// Get 返回当前 OCR 配置。ctx 用于取消与超时。
	// settings 为 nil 表示「尚未配置」，client 应当 fallback 到 YAML 默认值。
	Get(ctx context.Context) (*SettingsData, error)
	// Save 持久化新的配置（Upgrader / OCRTester.TestLLMWithSettings 会用到）。
	Save(ctx context.Context, settings *SettingsData) error
}

// SettingsData 是合并前各层配置的统一结构。
//
// 阶段 A+1：把它从 models.OCRSettings 解耦，让 ocr 包不依赖 internal/repository/postgres。
type SettingsData struct {
	Binary       string
	Audience     string
	URL          string
	Model        string
	ModelName    string
	AuthToken    string
	AuthHeader   string
	ExtraHeaders string
	UseAnthropic bool

	ExtraBody   string
	Language    string
	Background  string
	MaxTools    int
	ToolsPath   string
	ToolsConfig string
	McpServers  string

	Provider        string
	ReasoningMode   string
	ReasoningBudget int

	Concurrency    int
	MaxGitProcs    int
	TimeoutMinutes int

	TelemetryEnabled        bool
	TelemetryExporter       string
	TelemetryOTLPEndpoint   string
	TelemetryContentLogging bool

	ContextWindow   int
	MaxOutputTokens int

	// DefaultSeverity 是 OCRSettings.DefaultSeverity 字段的直传。
	// 阶段 A+1：完整字段映射表（与 models.OCRSettings 一对一）。
	DefaultSeverity string
}

// ToolSettings 是工具包配置接口（兼容旧 toolPackageSource）。
type ToolSettings interface {
	// List 返回所有可用的工具包。
	List(ctx context.Context) ([]ToolPackageData, error)
}

// ToolPackageData 是工具包配置的简化结构。
type ToolPackageData struct {
	Name     string
	ScanType string
	Enabled  bool
	Config   string // JSON 字符串
}

// SettingsSnapshot 是 Settings 接口的不可变快照实现，
// 用于测试与 fallback 场景（无 DB 时直接给一个固定值）。
type SettingsSnapshot struct {
	Data *SettingsData
}

// NewSettingsSnapshot 构造快照。
func NewSettingsSnapshot(data *SettingsData) *SettingsSnapshot {
	return &SettingsSnapshot{Data: data}
}

// Get 实现 Settings 接口。
func (s *SettingsSnapshot) Get(_ context.Context) (*SettingsData, error) {
	if s.Data == nil {
		return nil, fmt.Errorf("settings snapshot is nil")
	}
	return s.Data, nil
}

// Save 实现 Settings 接口（快照模式下 Save 是 no-op）。
func (s *SettingsSnapshot) Save(_ context.Context, _ *SettingsData) error {
	return nil
}

// 编译期断言：SettingsSnapshot 满足 Settings。
var _ Settings = (*SettingsSnapshot)(nil)
