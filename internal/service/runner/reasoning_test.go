package runner

import (
	"encoding/json"
	"testing"
)

func TestResolveExtraBody(t *testing.T) {
	tests := []struct {
		name          string
		extraBody     string
		reasoningMode string
		customBudget  int
		provider      string
		wantThinking  string
		wantBudget    int
		wantEmpty     bool
	}{
		// 预设档位（调大后的默认值）- Anthropic 格式
		{"light default budget=2000", "", "light", 0, "anthropic", "enabled", 2000, false},
		{"medium budget=8000", "", "medium", 0, "anthropic", "enabled", 8000, false},
		{"full budget=32000", "", "full", 0, "anthropic", "enabled", 32000, false},
		{"disabled", "", "disabled", 0, "anthropic", "disabled", 0, false},

		// 自定义 budget 覆盖预设
		{"custom budget overrides light", "", "light", 5000, "anthropic", "enabled", 5000, false},
		{"custom budget=10000", "", "medium", 10000, "anthropic", "enabled", 10000, false},
		{"custom mode with budget", "", "custom", 15000, "anthropic", "enabled", 15000, false},

		// 优先级
		{"explicit extra_body wins all", `{"custom":true}`, "light", 5000, "anthropic", "", 0, false},
		{"empty mode no intervention", "", "", 0, "anthropic", "", 0, true},
		{"unknown mode no intervention", "", "turbo", 0, "anthropic", "", 0, true},
		{"customBudget=0 falls back to preset", "", "light", 0, "anthropic", "enabled", 2000, false},

		// Qwen 格式
		{"qwen light", "", "light", 0, "dashscope", "enabled", 2000, false},
		{"qwen disabled", "", "disabled", 0, "qwen", "disabled", 0, false},

		// GLM 格式
		{"glm light", "", "light", 0, "z-ai", "enabled", 2000, false},
		{"glm disabled", "", "disabled", 0, "zhipu", "disabled", 0, false},

		// DeepSeek 格式
		{"deepseek light", "", "light", 0, "deepseek", "enabled", 2000, false},

		// MiMo 格式
		{"mimo light", "", "light", 0, "mimo", "enabled", 2000, false},
		{"mimo medium", "", "medium", 0, "xiaomi", "enabled", 8000, false},

		// 未知 provider 默认 Anthropic
		{"unknown provider defaults to anthropic", "", "light", 0, "unknown", "enabled", 2000, false},
		{"empty provider defaults to anthropic", "", "light", 0, "", "enabled", 2000, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ResolveExtraBody(tc.extraBody, tc.reasoningMode, tc.customBudget, tc.provider)

			if tc.wantEmpty {
				if got != "" {
					t.Fatalf("expected empty, got %s", got)
				}
				return
			}

			if tc.extraBody != "" && tc.extraBody != "null" {
				if got != tc.extraBody {
					t.Fatalf("expected explicit extra_body %s, got %s", tc.extraBody, got)
				}
				return
			}

			var parsed map[string]any
			if err := json.Unmarshal([]byte(got), &parsed); err != nil {
				t.Fatalf("invalid JSON: %s (err: %v)", got, err)
			}

			// 根据 provider 验证不同的 JSON 结构
			switch tc.provider {
			case "dashscope", "qwen", "aliyun":
				// Qwen 格式：{"enable_thinking":true/false,"thinking_budget":N}
				if tc.reasoningMode == "disabled" {
					if parsed["enable_thinking"] != false {
						t.Fatalf("qwen: expected enable_thinking=false, got %v", parsed["enable_thinking"])
					}
				} else {
					if parsed["enable_thinking"] != true {
						t.Fatalf("qwen: expected enable_thinking=true, got %v", parsed["enable_thinking"])
					}
					if int(parsed["thinking_budget"].(float64)) != tc.wantBudget {
						t.Fatalf("qwen: thinking_budget = %v, want %d", parsed["thinking_budget"], tc.wantBudget)
					}
				}
			default:
				// Anthropic 格式：{"thinking":{"type":"enabled"|"disabled","budget_tokens":N}}
				// GLM/DeepSeek/MiMo 格式：{"thinking":{"enabled":true/false,"budget_tokens":N}}
				thinking, ok := parsed["thinking"].(map[string]any)
				if !ok {
					t.Fatalf("missing thinking in %s", got)
				}
				
				// 检查 thinking 结构（支持 type 或 enabled 字段）
				if thinkingType, ok := thinking["type"]; ok {
					// Anthropic 格式
					if thinkingType != tc.wantThinking {
						t.Fatalf("thinking.type = %v, want %s", thinkingType, tc.wantThinking)
					}
				} else if thinkingEnabled, ok := thinking["enabled"]; ok {
					// GLM/DeepSeek/MiMo 格式
					wantEnabled := tc.wantThinking == "enabled"
					if thinkingEnabled != wantEnabled {
						t.Fatalf("thinking.enabled = %v, want %v", thinkingEnabled, wantEnabled)
					}
				} else {
					t.Fatalf("thinking missing 'type' or 'enabled' field in %s", got)
				}
				
				if tc.wantBudget > 0 {
					if int(thinking["budget_tokens"].(float64)) != tc.wantBudget {
						t.Fatalf("budget_tokens = %v, want %d", thinking["budget_tokens"], tc.wantBudget)
					}
				}
			}
		})
	}
}
