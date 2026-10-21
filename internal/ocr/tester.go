package ocr

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/yi-nology/code-review-service/internal/domain/ocrsettings"
	"github.com/yi-nology/code-review-service/internal/service/review/domain"

	"github.com/oekit/logx"
)

// resolveBinaryFrom 从 settings 解析二进制路径（fallback 到默认 "ocr"）。
// 这是 ocr/command.go、tester.go、versioner.go、provider_lister.go、rules_checker.go
// 5 处 resolveBinaryFrom 重复实现的单一真相。
func resolveBinaryFrom(s *ocrsettings.Settings, fallback Fallback) string {
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

// TestLLM 实现 domain.LLMTester：真实执行 `<binary> llm test` 并解析退出码与输出。
func (t *Tester) TestLLM(ctx context.Context) (*domain.LLMTestResult, error) {
	data, err := t.settings.Get(ctx)
	if err != nil {
		return nil, fmt.Errorf("读取 OCR 配置失败: %w", err)
	}
	if !data.enabled() {
		return &domain.LLMTestResult{Success: false, Message: "OCR 未启用"}, nil
	}

	cfg := t.resolver.ResolveFromSettings(data, OCROverrides{})
	return t.runLLMTest(ctx, cfg), nil
}

// TestLLMWithSettings 对指定的（尚未入库的）配置执行 LLM 连通测试。
func (t *Tester) TestLLMWithSettings(ctx context.Context, s *SettingsData) (*domain.LLMTestResult, error) {
	if s == nil {
		return &domain.LLMTestResult{Success: false, Message: "OCR 配置为空"}, nil
	}
	cfg := t.resolver.ResolveFromSettings(s, OCROverrides{})
	return t.runLLMTest(ctx, cfg), nil
}

// runLLMTest 执行 `<binary> llm test`：退出码 0 视为连通成功，非 0 视为失败。
// LLM 端点、模型、Token 通过 Config.AppendEnv 注入（与 review 子命令一致），
// 避免读取宿主 ~/.opencodereview/config.json 造成环境差异。
func (t *Tester) runLLMTest(ctx context.Context, cfg Config) *domain.LLMTestResult {
	binary := strings.TrimSpace(cfg.Binary)
	if binary == "" {
		binary = "ocr"
	}
	res, err := InternalRun(ctx, CommandSpec{
		Binary:  binary,
		Args:    []string{"llm", "test"},
		Env:     cfg.AppendEnv(childEnv(os.Environ(), cfg.hasExplicitLLMCredential())),
		Timeout: 60 * time.Second,
	})
	if err != nil && res == nil {
		return &domain.LLMTestResult{Success: false, Message: fmt.Sprintf("执行 %s llm test 失败: %v", binary, err)}
	}

	stdout := strings.TrimSpace(string(res.Stdout))
	stderr := strings.TrimSpace(string(res.Stderr))

	if res.ExitCode == 0 {
		return &domain.LLMTestResult{Success: true, Message: formatLLMTestSuccess(stdout)}
	}

	detail := strings.TrimSpace(strings.TrimPrefix(stderr, "Error:"))
	if detail == "" {
		detail = strings.TrimSpace(strings.TrimPrefix(stdout, "Error:"))
	}
	if res.TimedOut {
		if detail == "" {
			detail = "连接测试超时（60s）"
		} else {
			detail = "连接测试超时（60s）：" + detail
		}
	}
	if detail == "" {
		detail = fmt.Sprintf("%s llm test 退出码 %d", binary, res.ExitCode)
	}
	return &domain.LLMTestResult{Success: false, Message: detail}
}

// formatLLMTestSuccess 从 `ocr llm test` 成功输出中提取简洁的成功提示。
// 输出示例：
//
//	Source: OCR environment
//	URL:    https://token-plan-cn.xiaomimimo.com/v1
//	Model:  mimo-v2.5-pro
//	我是 open-code-review ...
//	✓ Connection test successful
func formatLLMTestSuccess(stdout string) string {
	model := ""
	for _, line := range strings.Split(stdout, "\n") {
		line = strings.TrimSpace(line)
		if model == "" && strings.HasPrefix(line, "Model:") {
			model = strings.TrimSpace(strings.TrimPrefix(line, "Model:"))
		}
	}
	if model != "" {
		return "连接测试成功（" + model + "）"
	}
	return "连接测试成功"
}

// SettingsData 上 enabled 判定（private helper，外部用 Enabled 字段更清晰）。
func (s *SettingsData) enabled() bool {
	// 阶段 A 占位：settings 没有 Enabled 字段；调用方应在 Save 前强制 enable
	return true
}

// logNoop 占位（让 logx 在 OCR 模块里仍能引用；阶段 A+3 删除）。
var _ = logx.Infof
