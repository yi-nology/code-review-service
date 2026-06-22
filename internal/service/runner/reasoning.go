package runner

import (
	"encoding/json"
	"strings"
)

// ReasoningMode 常量：控制推理模型（如 mimo-v2.5-pro / o1 / DeepSeek-R1）的行为。
const (
	// ReasoningDisabled 完全禁用推理，最快速度
	ReasoningDisabled = "disabled"
	// ReasoningLight 轻量推理，适合日常审查（默认）
	ReasoningLight = "light"
	// ReasoningMedium 中等推理，更深入分析复杂逻辑
	ReasoningMedium = "medium"
	// ReasoningFull 充分推理，最高质量，大 PR 慎用（慢）
	ReasoningFull = "full"
	// ReasoningCustom 使用 ReasoningBudget 字段的自定义值
	ReasoningCustom = "custom"
)

// reasoningBudget 映射推理模式 → 默认 thinking budget_tokens。
// 这些值针对代码审查场景调优：审查需要理解代码逻辑、并发、安全，需要足够推理空间。
var reasoningBudget = map[string]int{
	ReasoningDisabled: 0,
	ReasoningLight:    2000,  // 基础代码理解
	ReasoningMedium:   8000,  // 深入分析复杂逻辑
	ReasoningFull:     32000, // 充分推理，深度审查
}

// providerThinkingFormat 定义各 Provider 的 thinking JSON 格式
type providerThinkingFormat int

const (
	// formatAnthropic - Anthropic 格式：{"thinking":{"type":"enabled","budget_tokens":N}}
	formatAnthropic providerThinkingFormat = iota
	// formatOpenAIReasoning - OpenAI reasoning_effort 格式：{"reasoning_effort":"low"|"medium"|"high"}
	formatOpenAIReasoning
	// formatQwenThinking - Qwen/通义千问格式：{"enable_thinking":true,"thinking_budget":N}
	formatQwenThinking
	// formatGLMThinking - GLM/智谱格式：{"thinking":{"enabled":true,"budget_tokens":N}}
	formatGLMThinking
	// formatDeepSeekThinking - DeepSeek 格式：{"thinking":{"enabled":true,"budget_tokens":N}}
	formatDeepSeekThinking
	// formatMiMoThinking - MiMo 格式：{"thinking":{"enabled":true,"budget_tokens":N}}
	formatMiMoThinking
)

// providerFormatMap 映射 provider 名称 → thinking JSON 格式
var providerFormatMap = map[string]providerThinkingFormat{
	// Anthropic 系列
	"anthropic": formatAnthropic,
	// OpenAI 系列（o1, o3 等推理模型）
	"openai": formatOpenAIReasoning,
	// 通义千问系列（Qwen）
	"dashscope":  formatQwenThinking,
	"qwen":       formatQwenThinking,
	"aliyun":     formatQwenThinking,
	// 智谱 GLM 系列
	"z-ai":       formatGLMThinking,
	"zhipu":      formatGLMThinking,
	"bigmodel":   formatGLMThinking,
	// DeepSeek 系列
	"deepseek":   formatDeepSeekThinking,
	// MiMo 系列
	"mimo":       formatMiMoThinking,
	"xiaomi":     formatMiMoThinking,
}

// resolveReasoningBudget 根据 reasoningMode + customBudget 确定最终的 budget 值
func resolveReasoningBudget(reasoningMode string, customBudget int) int {
	mode := strings.ToLower(strings.TrimSpace(reasoningMode))
	if mode == "" {
		return 0
	}

	// disabled → 0
	if mode == ReasoningDisabled {
		return 0
	}

	// customBudget > 0 时覆盖预设值（适用于所有非 disabled 模式）
	if customBudget > 0 {
		return customBudget
	}

	// 预设模式
	budget, ok := reasoningBudget[mode]
	if !ok {
		return 0 // 未知模式
	}
	return budget
}

// ResolveExtraBody 根据 reasoningMode + customBudget + provider 解析最终的 extra_body JSON。
// 优先级：显式 extra_body > customBudget(>0) > 预设 mode budget > 不干预。
func ResolveExtraBody(extraBody, reasoningMode string, customBudget int, provider string) string {
	// 1. 显式 extra_body 最高优先（用户手写的 JSON 覆盖一切）
	if trimmed := strings.TrimSpace(extraBody); trimmed != "" && trimmed != "null" {
		return trimmed
	}

	// 2. 确定 budget
	budget := resolveReasoningBudget(reasoningMode, customBudget)

	// 3. budget=0 且 mode 不是 disabled 时不干预（除非显式 disabled）
	mode := strings.ToLower(strings.TrimSpace(reasoningMode))
	if budget == 0 && mode != ReasoningDisabled {
		return ""
	}

	// 4. 确定 provider 的 thinking 格式
	normalizedProvider := strings.ToLower(strings.TrimSpace(provider))
	format, ok := providerFormatMap[normalizedProvider]
	if !ok {
		// 默认使用 Anthropic 格式（最广泛兼容）
		format = formatAnthropic
	}

	// 5. 根据格式生成 JSON
	return generateThinkingJSON(format, budget, mode)
}

// generateThinkingJSON 根据格式和 budget 生成 thinking JSON
func generateThinkingJSON(format providerThinkingFormat, budget int, mode string) string {
	var cfg map[string]any

	switch format {
	case formatAnthropic:
		// Anthropic 格式：{"thinking":{"type":"enabled"|"disabled","budget_tokens":N}}
		if mode == ReasoningDisabled || budget == 0 {
			cfg = map[string]any{
				"thinking": map[string]any{
					"type": "disabled",
				},
			}
		} else {
			cfg = map[string]any{
				"thinking": map[string]any{
					"type":          "enabled",
					"budget_tokens": budget,
				},
			}
		}

	case formatOpenAIReasoning:
		// OpenAI 格式：{"reasoning_effort":"low"|"medium"|"high"}
		if mode == ReasoningDisabled || budget == 0 {
			cfg = map[string]any{
				"reasoning_effort": "low",
			}
		} else if budget <= 3000 {
			cfg = map[string]any{
				"reasoning_effort": "low",
			}
		} else if budget <= 12000 {
			cfg = map[string]any{
				"reasoning_effort": "medium",
			}
		} else {
			cfg = map[string]any{
				"reasoning_effort": "high",
			}
		}

	case formatQwenThinking:
		// Qwen 格式：{"enable_thinking":true,"thinking_budget":N}
		if mode == ReasoningDisabled || budget == 0 {
			cfg = map[string]any{
				"enable_thinking": false,
			}
		} else {
			cfg = map[string]any{
				"enable_thinking": true,
				"thinking_budget": budget,
			}
		}

	case formatGLMThinking:
		// GLM 格式：{"thinking":{"enabled":true,"budget_tokens":N}}
		if mode == ReasoningDisabled || budget == 0 {
			cfg = map[string]any{
				"thinking": map[string]any{
					"enabled": false,
				},
			}
		} else {
			cfg = map[string]any{
				"thinking": map[string]any{
					"enabled":       true,
					"budget_tokens": budget,
				},
			}
		}

	case formatDeepSeekThinking:
		// DeepSeek 格式：{"thinking":{"enabled":true,"budget_tokens":N}}
		if mode == ReasoningDisabled || budget == 0 {
			cfg = map[string]any{
				"thinking": map[string]any{
					"enabled": false,
				},
			}
		} else {
			cfg = map[string]any{
				"thinking": map[string]any{
					"enabled":       true,
					"budget_tokens": budget,
				},
			}
		}

	case formatMiMoThinking:
		// MiMo 格式：{"thinking":{"enabled":true,"budget_tokens":N}}
		if mode == ReasoningDisabled || budget == 0 {
			cfg = map[string]any{
				"thinking": map[string]any{
					"enabled": false,
				},
			}
		} else {
			cfg = map[string]any{
				"thinking": map[string]any{
					"enabled":       true,
					"budget_tokens": budget,
				},
			}
		}

	default:
		// 默认使用 Anthropic 格式
		if mode == ReasoningDisabled || budget == 0 {
			cfg = map[string]any{
				"thinking": map[string]any{
					"type": "disabled",
				},
			}
		} else {
			cfg = map[string]any{
				"thinking": map[string]any{
					"type":          "enabled",
					"budget_tokens": budget,
				},
			}
		}
	}

	out, err := json.Marshal(cfg)
	if err != nil {
		return `{"thinking":{"type":"disabled"}}`
	}
	return string(out)
}

// GetProviderThinkingFormat 获取 provider 的 thinking 格式（供外部查询）
func GetProviderThinkingFormat(provider string) string {
	normalizedProvider := strings.ToLower(strings.TrimSpace(provider))
	format, ok := providerFormatMap[normalizedProvider]
	if !ok {
		return "anthropic" // 默认
	}
	switch format {
	case formatAnthropic:
		return "anthropic"
	case formatOpenAIReasoning:
		return "openai"
	case formatQwenThinking:
		return "qwen"
	case formatGLMThinking:
		return "glm"
	case formatDeepSeekThinking:
		return "deepseek"
	case formatMiMoThinking:
		return "mimo"
	default:
		return "anthropic"
	}
}
