package modules

import (
	"github.com/yi-nology/code-review-service/biz/handler/progress_service"
	"github.com/yi-nology/code-review-service/internal/service/progress"

	"go.uber.org/fx"
)

// ProgressWSHandlerModule 提供 ProgressWSHandler。
// PR-16 修复：让 handler 可通过 fx Provide 注入，测试可 mock 替换。
var ProgressWSHandlerModule = fx.Options(
	fx.Provide(progress_service.NewProgressHandler),
	fx.Invoke(progress_service.InitHandler),
)

// _ 保留以避免 unused import
var _ = (*progress.ProgressManager)(nil)
