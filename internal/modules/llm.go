package modules

import (
	"github.com/yi-nology/code-review-service/internal/ocr"
	"github.com/yi-nology/code-review-service/internal/repository"
	"github.com/yi-nology/code-review-service/internal/service/llm"

	"go.uber.org/fx"
)

// LLMModule 提供统一的 LLM Provider（路径 B 地基）+ 调用指标（阶段 6）。
// 复用 OCR 配置（URL/Token/Model），无需新凭证；未配置时上层自动降级。
// 所有 LLM 调用经 MetricsProvider 包装，可统一观测调用/成功/失败/降级率。
//
// 阶段 A15：底层 provider 从 llm.NewDynamicProvider 切换到 ocr.LLMProvider。
// 适配路径：repos.OCRSettings (models.OCRSettings) → ocr.LLMRequest → ocr.LLMProvider → llm.Provider (via ocr.LLMProviderAdapter)
var LLMModule = fx.Options(
	fx.Provide(NewLLMProviderWithMetrics),
	fx.Provide(AsLLMProvider),
)

// NewLLMProviderWithMetrics 创建带指标统计的 Provider，同时返回指标对象。
//
// 阶段 A15：底层是 ocr.LLMProvider（HTTP 客户端在 ocr 包内），
// 通过 ocr.LLMProviderAdapter 适配为 llm.Provider（保持向后兼容）。
func NewLLMProviderWithMetrics(repos *repository.RepoContainer) (*llm.Metrics, *llm.MetricsProvider) {
	settings := ocr.NewSettingsAdapter(repos.OCRSettings)
	dynamic := ocr.NewLLMProvider(settings)
	adapter := ocr.WrapLLMProvider(dynamic)
	mp, m := llm.NewMetricsProvider(adapter)
	return m, mp
}

// AsLLMProvider 把 MetricsProvider 暴露为 llm.Provider 供 discussion/tech-lead 注入。
func AsLLMProvider(mp *llm.MetricsProvider) llm.Provider {
	return mp
}
