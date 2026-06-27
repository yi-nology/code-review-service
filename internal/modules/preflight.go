package modules

import (
	"context"
	"os/exec"

	"github.com/yi-nology/code-review-service/internal/readiness"
	"github.com/yi-nology/code-review-service/internal/repository"
	"github.com/yi-nology/code-review-service/internal/service/ocr"
	"github.com/yi-nology/code-review-service/internal/service/review/domain"

	"github.com/oekit/logx"
	"go.uber.org/fx"
)

// PreflightModule 在 fx 启动时执行依赖预检：OCR 二进制存在性 + LLM endpoint 可达性。
// 预检失败不阻断启动（return nil），而是通过 hlog 告警，并依赖 /health 的 checker
// 持续反映问题，让 k8s/运维能在 1 分钟内发现。这是本次 LLM DNS 事故的根因修复之一：
// 之前启动完全不验证 OCR/LLM，首次评审才发现故障。
var PreflightModule = fx.Options(fx.Invoke(RegisterPreflight))

func RegisterPreflight(lc fx.Lifecycle, repos *repository.RepoContainer, resolver *ocr.ConfigResolver) {
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			s, err := repos.OCRSettings.Get(ctx)
			if err != nil {
				logx.Errorf("[preflight] OCR 配置读取失败: %v", err)
				return nil
			}
			if s == nil || !s.Enabled {
				logx.Infof("[preflight] OCR 未启用，跳过 OCR/LLM 依赖检查")
				return nil
			}

			cfg, _ := resolver.Resolve(domain.OCROverrides{})
			binary := cfg.Binary
			if binary == "" {
				binary = "ocr"
			}
			if _, err := exec.LookPath(binary); err != nil {
				logx.Errorf("[preflight] OCR 二进制 %q 未在 PATH 找到: %v。代码评审功能将不可用，请在设置页配置正确的 OCR 二进制路径", binary, err)
			} else {
				logx.Infof("[preflight] OCR 二进制就绪: %s", binary)
			}

			if err := readiness.LLMEndpointCheck(repos, resolver)(ctx); err != nil {
				logx.Errorf("[preflight] LLM endpoint 检查失败: %v。评审将会失败，请检查 LLM 配置（URL/认证/网络）", err)
			} else {
				logx.Infof("[preflight] LLM endpoint 可达")
			}

			return nil
		},
	})
}
