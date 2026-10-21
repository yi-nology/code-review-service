package ocr

import (
	"context"
	"testing"
)

// stubProvider 返回固定内容的 LLM 桩。
type stubProvider struct{ content string }

func (s stubProvider) Complete(_ context.Context, _ CompleteRequest) (CompleteResponse, error) {
	return CompleteResponse{Content: s.content}, nil
}

func (stubProvider) Available(_ context.Context) bool { return true }

// TestCompleteJSONLenientParsing 验证 LLM 输出的 JSON 解析是**宽容**的。
//
// 解析委托 oekit/pkg/jsonrepair（围栏剥离 → 直解 → 语法级修复 →
// 散文里抽对象/数组）。此前的实现是手写的「剥围栏 + 取首个 {/[ 到末个 }/]」，
// 遇到尾逗号、全角冒号、散文包裹、未闭合括号就整段作废——而三个调用方
// 都把解析失败当「降级返回 nil」，等于静默丢掉整段评审产出。
//
// 表里后半段几例正是旧实现解析不了的形态，它们是这次重构的收益面。
func TestCompleteJSONLenientParsing(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected map[string]any
		wantErr  bool
	}{
		{
			name:     "pure json",
			input:    `{"a": 1, "b": "hello"}`,
			expected: map[string]any{"a": float64(1), "b": "hello"},
		},
		{
			name:     "json with markdown fences",
			input:    "```json\n{\"a\": 1}\n```",
			expected: map[string]any{"a": float64(1)},
		},
		{
			name:     "json with no language tag",
			input:    "```\n{\"a\": 1}\n```",
			expected: map[string]any{"a": float64(1)},
		},
		{
			name:    "invalid json",
			input:   "not json at all",
			wantErr: true,
		},
		{
			name:     "json with leading/trailing text",
			input:    "Here is the JSON: {\"a\": 1} hope it helps",
			expected: map[string]any{"a": float64(1)},
		},
		{
			// 尾逗号：旧实现整段作废。
			name:     "trailing comma",
			input:    "{\"a\": 1,}",
			expected: map[string]any{"a": float64(1)},
		},
		{
			// 全角冒号：模型在中文语境下的高频输出。
			name:     "full-width colon",
			input:    "{\"a\"：1}",
			expected: map[string]any{"a": float64(1)},
		},
		{
			// 散文 + 围栏 + 尾逗号叠加。
			name:     "prose with fence and trailing comma",
			input:    "分析结果如下：\n```json\n{\"a\": 1,}\n```\n以上。",
			expected: map[string]any{"a": float64(1)},
		},
		{
			// 未闭合括号（输出被截断）。
			name:     "unclosed brace",
			input:    "{\"a\": 1",
			expected: map[string]any{"a": float64(1)},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got map[string]any
			err := CompleteJSON(context.Background(), stubProvider{content: tt.input},
				CompleteRequest{UserPrompt: "test"}, &got)
			if (err != nil) != tt.wantErr {
				t.Errorf("CompleteJSON() err = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}
			if len(got) != len(tt.expected) {
				t.Errorf("CompleteJSON() got %v, want %v", got, tt.expected)
			}
		})
	}
}

// TestCompleteJSONTopLevelArray 验证顶层是数组时也能解析。
//
// requirements 澄清提问要的正是数组（[]ClarifyingQuestion），旧的
// 「取首个 {/[ 到末个 }/]」在数组里混着对象时容易切错。
func TestCompleteJSONTopLevelArray(t *testing.T) {
	var got []struct {
		Question string `json:"question"`
	}
	err := CompleteJSON(context.Background(),
		stubProvider{content: "```json\n[{\"question\": \"为什么这样写?\"}]\n```"},
		CompleteRequest{UserPrompt: "test"}, &got)
	if err != nil {
		t.Fatalf("CompleteJSON() err = %v", err)
	}
	if len(got) != 1 || got[0].Question != "为什么这样写?" {
		t.Errorf("got %+v", got)
	}
}

// TestNewNoopProvider 验证 NoopProvider 行为。
func TestNewNoopProvider(t *testing.T) {
	np := NewNoopProvider()

	// Available 始终 false
	if np.Available(context.Background()) {
		t.Errorf("expected NoopProvider.Available=false")
	}

	// Complete 返回 ErrNotConfigured
	_, err := np.Complete(context.Background(), CompleteRequest{UserPrompt: "test"})
	if err == nil {
		t.Errorf("expected error from NoopProvider.Complete")
	}
}

// TestLLMProvider_CompleteJSON 验证 CompleteJSON 便捷方法。
// TestLLMProvider_Reset 验证 Reset 清除缓存。
// TestLLMProvider_HTTPClient 验证 HTTP client 配置。
// TestLLMProvider_GetSettingsCached 验证缓存命中。
// TestLLMProvider_Available_CacheHit 验证 Available 走缓存。
