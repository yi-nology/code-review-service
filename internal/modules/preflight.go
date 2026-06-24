package modules

import (
	"context"
	"os/exec"

	"github.com/yi-nology/code-review-service/internal/repository"
	"github.com/yi-nology/code-review-service/internal/service/ocr"
	"github.com/yi-nology/code-review-service/internal/service/review"

	"github.com/oekit/logx"
	"go.uber.org/fx"
)

// PreflightModule 在 fx 启动时执行依赖预检：OCR 二进制存在性 + LLM endpoint 可达性。
// 预检失败不阻断启动（return nil），而是通过 hlog 告警，并依赖 /health 的 checker
// 持续反映问题，让 k8s/运维能在 1 分钟内发现。这是本次 LLM DNS 事故的根因修复之一：
// 之前启动完全不验证 OCR/LLM，首次评审才发现故障。
var PreflightModule = fx.Invoke(RegisterPreflight)

func RegisterPreflight(lc fx.Lifecycle, repos *repository.RepoContainer, resolver *ocr.ConfigResolver) {
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			cfg, _ := resolver.Resolve(review.OCROverrides{})
			binary := cfg.Binary
			if binary == "" {
				binary = "ocr"
			}
			if _, err := exec.LookPath(binary); err != nil {
				logx.Errorf("[preflight] OCR 二进制 %q 未在 PATH 找到: %v。代码评审功能将不可用，请在设置页配置正确的 OCR 二进制路径", binary, err)
			} else {
				logx.Infof("[preflight] OCR 二进制就绪: %s", binary)
			}

			if s, err := repos.OCRSettings.Get(ctx); err == nil && s != nil && s.Enabled {
				if err := llmEndpointCheck(repos, resolver)(ctx); err != nil {
					logx.Errorf("[preflight] LLM endpoint 检查失败: %v。评审将会失败，请检查 LLM 配置（URL/认证/网络）", err)
				} else {
					logx.Infof("[preflight] LLM endpoint 可达")
				}
			}

			return nil
		},
	})
}
