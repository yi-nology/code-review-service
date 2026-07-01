package modules

import (
	"github.com/yi-nology/code-review-service/internal/config"
	"github.com/yi-nology/code-review-service/internal/ocr"

	"go.uber.org/fx"
)

// RunnerModule 提供 ocr.Runner（实现 domain.Runner/PreviewRunner/ScanRunner）。
//
// 阶段 A15 part 3：完全从 runner 包脱离。所有调用方拿 *ocr.Runner。
var RunnerModule = fx.Options(
	fx.Provide(NewOpenCodeReviewRunner),
)

// NewOpenCodeReviewRunner 从 YAML fallback config 构造 ocr.Runner。
func NewOpenCodeReviewRunner(cfg *config.Config) *ocr.Runner {
	return ocr.NewRunner(ocr.Fallback{
		Binary:          cfg.OCR.Binary,
		Audience:        "human",
		RulePath:        cfg.OCR.Rule,
		DefaultSeverity: "major",
	}.ToConfig())
}