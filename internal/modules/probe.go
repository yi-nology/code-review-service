package modules

import (
	"context"
	"time"

	"github.com/yi-nology/code-review-service/internal/repository"
	"github.com/yi-nology/code-review-service/internal/service/ocr"

	"github.com/oekit/concurrency/async"
	"github.com/cloudwego/hertz/pkg/common/hlog"
	"go.uber.org/fx"
)

// LLMProbeModule 周期探活 LLM endpoint（#8），主动发现故障而非等 /health 请求或评审失败。
// 复用 health.go 的 llmEndpointCheck（轻量 TCP 拨号），失败时 hlog.Errorf 告警。
var LLMProbeModule = fx.Invoke(RegisterLLMProbe)

func RegisterLLMProbe(lc fx.Lifecycle, repos *repository.RepoContainer, resolver *ocr.ConfigResolver) {
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			async.GoSafe(func() {
				ticker := time.NewTicker(30 * time.Second)
				defer ticker.Stop()
				check := llmEndpointCheck(repos, resolver)
				for {
					select {
					case <-ctx.Done():
						return
					case <-ticker.C:
						if err := check(ctx); err != nil {
							hlog.Errorf("[llm-probe] LLM endpoint 探活失败: %v（评审将会失败，请检查 LLM 配置/网络）", err)
						}
					}
				}
			})
			return nil
		},
	})
}
