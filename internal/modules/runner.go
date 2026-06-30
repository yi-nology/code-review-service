package modules

import (
	"github.com/yi-nology/code-review-service/internal/config"
	"github.com/yi-nology/code-review-service/internal/ocr"
	"github.com/yi-nology/code-review-service/internal/repository"

	"go.uber.org/fx"
)

// RunnerModule 提供 ocr.Runner（实现 domain.Runner/PreviewRunner/ScanRunner）。
//
// 阶段 A15：从 runner.NewOpenCodeReviewRunner 切换到 ocr.NewRunner。
// DynamicOpenCodeReviewRunner 的合并逻辑被 ocr.ConfigResolver 吸收。
var RunnerModule = fx.Options(
	fx.Provide(NewOpenCodeReviewRunner),
)

// NewOpenCodeReviewRunner 从 YAML fallback config 构造 ocr.Runner。
func NewOpenCodeReviewRunner(cfg *config.Config) *ocr.Runner {
	fallback := ocr.Fallback{
		Binary:          cfg.OCR.Binary,
		Audience:        "human",
		RulePath:        cfg.OCR.Rule,
		DefaultSeverity: "major",
	}
	return ocr.NewRunner(fallback.ToConfig())
}

// NewDynamicOpenCodeReviewRunner 兼容老调用点（已废弃）。
//
// Deprecated: 阶段 A15 后删除。改用 ocr.NewRunner + ocr.ConfigResolver 组合。
// 保留此函数是为了让 modules/review.go 不立即崩；阶段 A15 末尾 fx.Provide 改回 ocr.NewRunner。
func NewDynamicOpenCodeReviewRunner(
	repos *repository.RepoContainer,
) *ocr.Runner {
	settings := ocr.NewSettingsAdapter(repos.OCRSettings)
	resolver := ocr.NewConfigResolver(settings, ocr.Defaults(), nil)
	// 阶段 A15 占位：用 ocr.Runner 作为统一 runner，Config 直接来自 fallback（不读 DB）。
	_ = resolver
	_ = settings
	return ocr.NewRunner(ocr.Defaults().ToConfig())
}