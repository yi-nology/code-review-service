package modules

import (
	"github.com/yi-nology/code-review-service/internal/ocr"

	"go.uber.org/fx"
)

// RunnerModule 提供 ocr.Runner（实现 domain.Runner/PreviewRunner/ScanRunner）。
var RunnerModule = fx.Options(
	fx.Provide(NewOpenCodeReviewRunner),
)

// NewOpenCodeReviewRunner 构造 ocr.Runner，注入 ConfigResolver 以便后台从 DB 刷新配置。
func NewOpenCodeReviewRunner(resolver *ocr.ConfigResolver) *ocr.Runner {
	return ocr.NewRunner(ocr.Defaults().ToConfig()).WithResolver(resolver)
}
