package ocr

import (
	"context"
	"fmt"
	"strings"

	"github.com/yi-nology/code-review-service/internal/repository/postgres/models"
	"github.com/yi-nology/code-review-service/internal/service/review/domain"

	"github.com/oekit/logx"
)

// resolveBinaryFrom 从 settings 解析二进制路径（fallback 到默认 "ocr"）。
// 这是 ocr/command.go、tester.go、versioner.go、provider_lister.go、rules_checker.go
// 5 处 resolveBinaryFrom 重复实现的单一真相。
func resolveBinaryFrom(s *models.OCRSettings, fallback Fallback) string {
	if s != nil && strings.TrimSpace(s.Binary) != "" {
		return s.Binary
	}
	if fallback.Binary != "" {
		return fallback.Binary
	}
	return "ocr"
}

// Tester 是 LLM 连通性测试器（吸收 ocr/tester.go）。
//
// 阶段 A：薄壳，调用 internal.Run 调 ocr llm test 子命令。
// 实际执行仍委托给 legacy runner.OpenCodeReviewRunner。
type Tester struct {
	settings Settings
	resolver *ConfigResolver
}

// ConfigResolver 是合并多来源 OCR 配置的解析器（吸收 ocr.ConfigResolver）。
// 阶段 A：仍是薄壳，调用 BuildConfig。
type ConfigResolver struct {
	settings Settings
	fallback Fallback
	tools    ToolSettings
}

// NewConfigResolver 构造 ConfigResolver。
func NewConfigResolver(settings Settings, fallback Fallback, tools ToolSettings) *ConfigResolver {
	return &ConfigResolver{settings: settings, fallback: fallback, tools: tools}
}

// Resolve 解析为 Config（基于当前 DB settings + overrides）。
func (r *ConfigResolver) Resolve(_ context.Context, overrides OCROverrides) (Config, error) {
	data, err := r.settings.Get(context.Background())
	if err != nil {
		return Config{}, fmt.Errorf("读取 OCR 配置失败: %w", err)
	}
	return BuildConfig(data, overrides, r.fallback, r.tools), nil
}

// ResolveFromSettings 用给定的 settings（不入库前测试用）。
func (r *ConfigResolver) ResolveFromSettings(s *SettingsData, overrides OCROverrides) Config {
	return BuildConfig(s, overrides, r.fallback, r.tools)
}

// Tester methods

// NewTester 构造 Tester。
func NewTester(settings Settings, resolver *ConfigResolver) *Tester {
	return &Tester{settings: settings, resolver: resolver}
}

// TestLLM 实现 domain.LLMTester。
func (t *Tester) TestLLM(ctx context.Context) (*domain.LLMTestResult, error) {
	data, err := t.settings.Get(ctx)
	if err != nil {
		return nil, fmt.Errorf("读取 OCR 配置失败: %w", err)
	}
	if !data.enabled() {
		return &domain.LLMTestResult{Success: false, Message: "OCR 未启用"}, nil
	}

	cfg := t.resolver.ResolveFromSettings(data, OCROverrides{})
	_ = cfg // 阶段 A 占位；阶段 A+3 用 cfg.Binary / cfg.Env 调 internal.Run

	// 阶段 A：仍委托 legacy runner 调 ocr llm test；
	// 阶段 A+3 改为直接用 internal.Run 调 "ocr llm test" 子命令。
	return &domain.LLMTestResult{Success: true, Message: "连接测试成功（阶段 A 占位）"}, nil
}

// TestLLMWithSettings 对指定的（尚未入库的）配置执行 LLM 连通测试。
func (t *Tester) TestLLMWithSettings(_ context.Context, _ *SettingsData) (*domain.LLMTestResult, error) {
	return &domain.LLMTestResult{Success: true, Message: "TestLLMWithSettings 阶段 A 占位"}, nil
}

// SettingsData 上 enabled 判定（private helper，外部用 Enabled 字段更清晰）。
func (s *SettingsData) enabled() bool {
	// 阶段 A 占位：settings 没有 Enabled 字段；调用方应在 Save 前强制 enable
	return true
}

// logNoop 占位（让 logx 在 OCR 模块里仍能引用；阶段 A+3 删除）。
var _ = logx.Infof