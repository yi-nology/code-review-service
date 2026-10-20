package ocr

import (
	"context"
	"testing"
)

// TestParseJSONLoose 验证 parseJSONLoose 函数能处理各种 JSON 格式。
func TestParseJSONLoose(t *testing.T) {
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
			wantErr:  false,
		},
		{
			name:     "json with markdown fences",
			input:    "```json\n{\"a\": 1}\n```",
			expected: map[string]any{"a": float64(1)},
			wantErr:  false,
		},
		{
			name:     "json with no language tag",
			input:    "```\n{\"a\": 1}\n```",
			expected: map[string]any{"a": float64(1)},
			wantErr:  false,
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
			wantErr:  false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got map[string]any
			err := parseJSONLoose(tt.input, &got)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseJSONLoose() err = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}
			if len(got) != len(tt.expected) {
				t.Errorf("parseJSONLoose() got %v, want %v", got, tt.expected)
			}
		})
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
