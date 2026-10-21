package ocr

import (
	"encoding/json"
	"strings"
)

// Reasoning mode constants
const (
	ReasoningDisabled = "disabled"
	ReasoningLight    = "light"
	ReasoningMedium   = "medium"
	ReasoningFull     = "full"
)

// reasoningBudget 映射推理模式 → 默认 thinking budget_tokens
var reasoningBudget = map[string]int{
	ReasoningDisabled: 0,
	ReasoningLight:    2000,
	ReasoningMedium:   8000,
	ReasoningFull:     32000,
}

// providerFormat 定义各 Provider 的 thinking JSON 格式
type providerFormat int

const (
	formatAnthropic providerFormat = iota
	formatOpenAIReasoning
	formatQwenThinking
	formatGLMThinking
	formatDeepSeekThinking
	formatMiMoThinking
)

var providerFormatMap = map[string]providerFormat{
	"anthropic": formatAnthropic,
	"openai":    formatOpenAIReasoning,
	"dashscope": formatQwenThinking,
	"qwen":      formatQwenThinking,
	"aliyun":    formatQwenThinking,
	"z-ai":      formatGLMThinking,
	"zhipu":     formatGLMThinking,
	"bigmodel":  formatGLMThinking,
	"deepseek":  formatDeepSeekThinking,
	"mimo":      formatMiMoThinking,
	"xiaomi":    formatMiMoThinking,
}

// ResolveExtraBody 根据 reasoningMode + customBudget + provider 解析最终的 extra_body JSON。
// 吸收 runner.ResolveExtraBody。
func ResolveExtraBody(extraBody, reasoningMode string, customBudget int, provider string) string {
	// 1. 显式 extra_body 最高优先
	if trimmed := strings.TrimSpace(extraBody); trimmed != "" && trimmed != "null" {
		return trimmed
	}
	// 2. 确定 budget
	budget := resolveReasoningBudget(reasoningMode, customBudget)
	// 3. budget=0 且 mode 不是 disabled 时不干预
	mode := strings.ToLower(strings.TrimSpace(reasoningMode))
	if budget == 0 && mode != ReasoningDisabled {
		return ""
	}
	// 4. 确定 provider 的 thinking 格式
	normalizedProvider := strings.ToLower(strings.TrimSpace(provider))
	format, ok := providerFormatMap[normalizedProvider]
	if !ok {
		format = formatAnthropic
	}
	// 5. 根据格式生成 JSON
	return generateThinkingJSON(format, budget, mode)
}

func resolveReasoningBudget(reasoningMode string, customBudget int) int {
	mode := strings.ToLower(strings.TrimSpace(reasoningMode))
	if mode == "" {
		return 0
	}
	if mode == ReasoningDisabled {
		return 0
	}
	if customBudget > 0 {
		return customBudget
	}
	budget, ok := reasoningBudget[mode]
	if !ok {
		return 0
	}
	return budget
}

func generateThinkingJSON(format providerFormat, budget int, mode string) string {
	var cfg map[string]any

	switch format {
	case formatAnthropic:
		if mode == ReasoningDisabled || budget == 0 {
			cfg = map[string]any{"thinking": map[string]any{"type": "disabled"}}
		} else {
			cfg = map[string]any{"thinking": map[string]any{"type": "enabled", "budget_tokens": budget}}
		}
	case formatOpenAIReasoning:
		if mode == ReasoningDisabled || budget == 0 {
			cfg = map[string]any{"reasoning_effort": "low"}
		} else if budget <= 3000 {
			cfg = map[string]any{"reasoning_effort": "low"}
		} else if budget <= 12000 {
			cfg = map[string]any{"reasoning_effort": "medium"}
		} else {
			cfg = map[string]any{"reasoning_effort": "high"}
		}
	case formatQwenThinking:
		if mode == ReasoningDisabled || budget == 0 {
			cfg = map[string]any{"enable_thinking": false}
		} else {
			cfg = map[string]any{"enable_thinking": true, "thinking_budget": budget}
		}
	default:
		if mode == ReasoningDisabled || budget == 0 {
			cfg = map[string]any{"thinking": map[string]any{"enabled": false}}
		} else {
			cfg = map[string]any{"thinking": map[string]any{"enabled": true, "budget_tokens": budget}}
		}
	}

	out, err := json.Marshal(cfg)
	if err != nil {
		return `{"thinking":{"type":"disabled"}}`
	}
	return string(out)
}
