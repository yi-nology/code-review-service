package ocr

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/yi-nology/code-review-service/internal/service/review"
)

type RulesChecker struct {
	settings    SettingsSource
	globalRules GlobalRulesSource
	resolver    *ConfigResolver
}

func NewRulesChecker(settings SettingsSource, globalRules GlobalRulesSource, resolver *ConfigResolver) *RulesChecker {
	return &RulesChecker{settings: settings, globalRules: globalRules, resolver: resolver}
}

func (rc *RulesChecker) RulesCheck(ctx context.Context, path string) (*review.RulesCheckResult, error) {
	s, err := rc.settings.Get(ctx)
	if err != nil {
		return nil, fmt.Errorf("读取 OCR 配置失败: %w", err)
	}

	binary := resolveBinaryFrom(s, rc.resolver.fallback)

	args := []string{"rules", "check"}

	if rc.globalRules != nil {
		rulePath, cleanup, ruleErr := resolveGlobalRuleFile(ctx, rc.globalRules)
		if ruleErr == nil && rulePath != "" {
			args = append(args, "--rule", rulePath)
			if cleanup != nil {
				defer cleanup()
			}
		}
	}

	args = append(args, path)

	result, err := runOCRCommand(ctx, binary, args, nil)
	if err != nil {
		msg := strings.TrimSpace(result.stderr)
		if msg == "" {
			msg = err.Error()
		}
		return &review.RulesCheckResult{Applicable: false, Message: msg}, nil
	}

	output := strings.TrimSpace(result.stdout)
	return &review.RulesCheckResult{
		Applicable: output != "",
		Rule:       output,
		Message:    "规则匹配成功",
	}, nil
}

var _ review.RulesChecker = (*RulesChecker)(nil)

func resolveGlobalRuleFile(ctx context.Context, globalRules GlobalRulesSource) (string, func(), error) {
	if globalRules == nil {
		return "", nil, nil
	}

	rules, err := globalRules.Get(ctx)
	if err != nil {
		return "", nil, nil //nolint:nilerr
	}
	if rules == nil || len(rules.Rules) == 0 {
		return "", nil, nil
	}

	f, err := os.CreateTemp("", "ocr-rules-*.json")
	if err != nil {
		return "", nil, err
	}

	ruleJSON := fmt.Sprintf(`{"rules":%s}`, string(rules.Rules))
	if _, err := f.WriteString(ruleJSON); err != nil {
		_ = f.Close()
		_ = os.Remove(f.Name())
		return "", nil, err
	}
	_ = f.Close()

	cleanup := func() { _ = os.Remove(f.Name()) }
	return f.Name(), cleanup, nil
}
