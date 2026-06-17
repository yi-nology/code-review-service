package ocr

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/yi-nology/code-review-service/internal/service/review"
)

type Tester struct {
	settings SettingsSource
	resolver *ConfigResolver
}

func NewTester(settings SettingsSource, resolver *ConfigResolver) *Tester {
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
	env = appendLLMEnv(env, cfg)

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
