package modules

import (
	"github.com/yi-nology/code-review-service/internal/ocr"
	"github.com/yi-nology/code-review-service/internal/repository"

	"go.uber.org/fx"
)

// LLMModule 提供统一的 LLM Provider + 调用指标。
// 复用 OCR 配置（URL/Token/Model），无需新凭证；未配置时上层自动降级。
//
// 阶段 A10：底层 provider 从 llm.NewDynamicProvider 切换到 ocr.LLMProvider。
// 所有 LLM 调用经 MetricsProvider 包装，可统一观测调用/成功/失败/降级率。
var LLMModule = fx.Options(
	fx.Provide(NewLLMProviderWithMetrics),
	fx.Provide(AsLLMProvider),
)

// NewLLMProviderWithMetrics 创建带指标统计的 Provider，同时返回指标对象。
func NewLLMProviderWithMetrics(repos *repository.RepoContainer) (*ocr.Metrics, *ocr.MetricsProvider) {
	settings := ocr.NewSettingsAdapter(repos.OCRSettings)
	dynamic := ocr.NewLLMProvider(settings)
	adapter := ocr.WrapLLMProvider(dynamic)
	mp, m := ocr.NewMetricsProvider(adapter)
	return m, mp
}

// AsLLMProvider 把 MetricsProvider 暴露为 Provider 供 discussion/tech-lead 注入。
func AsLLMProvider(mp *ocr.MetricsProvider) ocr.Provider {
	return mp
}
