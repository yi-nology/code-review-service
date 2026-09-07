package modules

import (
	"github.com/yi-nology/code-review-service/internal/ocr"
	"github.com/yi-nology/code-review-service/internal/repository"

	"go.uber.org/fx"
)

// LLMModule 提供统一的 LLM Provider + 调用指标。
// 复用 OCR 配置（URL/Token/Model），无需新凭证；未配置时上层自动降级。
//
// ocr.Provider 用于：discussion 多轮讨论、tech-lead 分析等 post-OCR 阶段。
// OCR binary 自身的 LLM 调用由 ocr-engine 通过 OCR_LLM_* 环境变量管理，
// 不经过此模块。
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
