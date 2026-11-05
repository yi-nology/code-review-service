package modules

import (
	"context"
	"time"

	"github.com/yi-nology/code-review-service/internal/ocr"
	"github.com/yi-nology/code-review-service/internal/readiness"

	"github.com/oekit/concurrency/async"
	"github.com/oekit/logx"
	"go.uber.org/fx"
)

// LLMProbeModule 周期探活 LLM endpoint（#8），主动发现故障而非等 /health 请求或评审失败。
// 复用 health.go 的 llmEndpointCheck（轻量 TCP 拨号），失败时 logx.Errorf 告警。
var LLMProbeModule = fx.Options(fx.Invoke(RegisterLLMProbe))

// RegisterLLMProbe 收口为 2 个参数：lc + *ocr.Registry。
// 之前 3 个参数（lc + repos + resolver）。
func RegisterLLMProbe(lc fx.Lifecycle, ocrReg *ocr.Registry) {
	repos := ocrReg.Repos()
	resolver := ocrReg.Resolver()

	var probeCancel context.CancelFunc
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			probeCtx, cancel := context.WithCancel(context.Background())
			probeCancel = cancel
			async.GoSafe(func() {
				ticker := time.NewTicker(30 * time.Second)
				defer ticker.Stop()
				check := readiness.LLMEndpointCheck(repos, resolver)
				// 连续失败阈值：探活是趋势信号，不被单次抖动打红
				//（生产实测 DNS 路径偶发抖动、端点实际可达——单次失败降 Warn
				// 留痕，连续 3 次（≈90s）仍失败才 Error 红灯告警；成功即清零）。
				const failThreshold = 3
				failStreak := 0
				for {
					select {
					case <-probeCtx.Done():
						return
					case <-ticker.C:
						if err := check(probeCtx); err != nil {
							failStreak++
							if failStreak >= failThreshold {
								logx.Errorf("[llm-probe] LLM endpoint 探活连续失败 %d 次: %v（评审将会失败，请检查 LLM 配置/网络）", failStreak, err)
							} else {
								logx.Warnf("[llm-probe] LLM endpoint 探活失败（%d/%d，疑似抖动）: %v", failStreak, failThreshold, err)
							}
						} else {
							failStreak = 0
						}
					}
				}
			})
			return nil
		},
		OnStop: func(ctx context.Context) error {
			if probeCancel != nil {
				probeCancel()
			}
			logx.Info("llm endpoint probe stopped")
			return nil
		},
	})
}
