package modules

import (
	"context"

	"github.com/oekit/logx"
	"github.com/yi-nology/code-review-service/internal/ocr"
	"github.com/yi-nology/code-review-service/internal/repository"

	"go.uber.org/fx"
)

// LLMModule 提供统一的 LLM Provider + 调用指标。
// 复用 OCR 配置（URL/Token/Model），无需新凭证；未配置时上层自动降级。
//
// 使用 agentkit/llm.Resilient 作为后端（自动获得重试/限速/熔断/fitInput/降级链）。
// ocr.Provider 用于：discussion 多轮讨论、tech-lead 分析、Judge 二次判断等 post-OCR 阶段。
var LLMModule = fx.Options(
	fx.Provide(NewLLMProviderWithMetrics),
	fx.Provide(AsLLMProvider),
)

// NewLLMProviderWithMetrics 创建基于 agentkit 的 Provider + 指标统计。
// 使用 BuildGeneratorFromSettings → AgentKitProvider 替代旧的手撸 HTTP LLMProvider。
func NewLLMProviderWithMetrics(repos *repository.RepoContainer) (*ocr.Metrics, *ocr.MetricsProvider) {
	settings := ocr.NewSettingsAdapter(repos.OCRSettings)
	gen, err := ocr.BuildGeneratorFromSettings(context.Background(), settings)
	if err != nil {
		logx.Warnf("build agentkit generator failed, falling back to noop: %v", err)
	}
	provider := ocr.NewAgentKitProvider(gen, settings)
	mp, m := ocr.NewMetricsProvider(provider)
	return m, mp
}

// AsLLMProvider 把 MetricsProvider 暴露为 Provider 供 discussion/tech-lead 注入。
func AsLLMProvider(mp *ocr.MetricsProvider) ocr.Provider {
	return mp
}
