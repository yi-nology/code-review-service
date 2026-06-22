package ocr

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/yi-nology/code-review-service/internal/repository/postgres/models"
	"github.com/yi-nology/code-review-service/internal/service/review"
	"github.com/yi-nology/code-review-service/internal/service/runner"
)

type Tester struct {
	settings settingsSource
	resolver *ConfigResolver
}

func NewTester(settings settingsSource, resolver *ConfigResolver) *Tester {
	return &Tester{settings: settings, resolver: resolver}
}

func (t *Tester) TestLLM(ctx context.Context) (*review.LLMTestResult, error) {
	s, err := t.settings.Get(ctx)
	if err != nil {
		return nil, fmt.Errorf("读取 OCR 配置失败: %w", err)
	}
	if !s.Enabled {
		return &review.LLMTestResult{Success: false, Message: "OCR 未启用"}, nil
	}

	cfg := t.resolver.ResolveFromSettings(s, review.OCROverrides{})

	args := []string{"llm", "test"}
	env := os.Environ()
	env = runner.AppendLLMEnv(env, cfg)

	result, err := runOCRCommand(ctx, cfg.Binary, args, env)
	if err != nil {
		msg := strings.TrimSpace(result.stderr)
		if msg == "" {
			msg = err.Error()
		}
		return &review.LLMTestResult{Success: false, Message: msg}, nil
	}

	output := strings.TrimSpace(result.stdout)
	if output == "" {
		output = "连接测试成功"
	}
	return &review.LLMTestResult{Success: true, Message: output}, nil
}

// TestLLMWithSettings 对指定的（尚未入库的）配置执行 LLM 连通测试。
// 用于 UpdateOCRSettings 在保存前预检，防止错误配置入库后导致评审静默失败。
// 注意：该方法不进入 review.LLMTester interface，handler 通过类型断言调用，
// 这样测试用的 mock 无需实现该方法（断言失败即跳过预检）。
func (t *Tester) TestLLMWithSettings(ctx context.Context, s *models.OCRSettings) (*review.LLMTestResult, error) {
	if s == nil {
		return nil, fmt.Errorf("settings 为空")
	}
	if !s.Enabled {
		return &review.LLMTestResult{Success: true, Message: "OCR 未启用，跳过 LLM 测试"}, nil
	}

	cfg := t.resolver.ResolveFromSettings(s, review.OCROverrides{})

	args := []string{"llm", "test"}
	env := os.Environ()
	env = runner.AppendLLMEnv(env, cfg)

	result, err := runOCRCommand(ctx, cfg.Binary, args, env)
	if err != nil {
		msg := strings.TrimSpace(result.stderr)
		if msg == "" {
			msg = err.Error()
		}
		return &review.LLMTestResult{Success: false, Message: msg}, nil
	}

	output := strings.TrimSpace(result.stdout)
	if output == "" {
		output = "连接测试成功"
	}
	return &review.LLMTestResult{Success: true, Message: output}, nil
}

var _ review.LLMTester = (*Tester)(nil)
