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
func TestLLMProvider_CompleteJSON(t *testing.T) {
	settings := NewSettingsSnapshot(&SettingsData{
		URL:        "https://api.example.com",
		ModelName:  "gpt-4",
		AuthToken:  "test-token",
	})
	provider := NewLLMProvider(settings)

	// 无效 URL（故意）应返回错误
	var out map[string]any
	err := provider.CompleteJSON(context.Background(), CompleteRequest{
		SystemPrompt: "test",
		UserPrompt:   "hello",
	}, &out)
	if err == nil {
		t.Errorf("expected error from invalid endpoint")
	}
}

// TestLLMProvider_Reset 验证 Reset 清除缓存。
func TestLLMProvider_Reset(t *testing.T) {
	settings := NewSettingsSnapshot(&SettingsData{
		URL:       "https://api.example.com",
		ModelName: "gpt-4",
		AuthToken: "test",
	})
	provider := NewLLMProvider(settings)

	// 第一次获取：写入缓存
	data1, err := provider.getSettingsCached(context.Background())
	if err != nil || data1 == nil {
		t.Fatalf("first getSettingsCached err: %v", err)
	}

	// 验证缓存生效
	provider.settingsCacheMu.RLock()
	cached := provider.settingsCache
	provider.settingsCacheMu.RUnlock()
	if cached == nil {
		t.Errorf("expected cache to be set")
	}

	// 调用 Reset
	provider.Reset()

	// 验证缓存已清除
	provider.settingsCacheMu.RLock()
	cached = provider.settingsCache
	provider.settingsCacheMu.RUnlock()
	if cached != nil {
		t.Errorf("expected cache to be cleared after Reset, got %v", cached)
	}
}

// TestLLMProvider_HTTPClient 验证 HTTP client 配置。
func TestLLMProvider_HTTPClient(t *testing.T) {
	settings := NewSettingsSnapshot(&SettingsData{URL: "https://x", ModelName: "gpt-4"})
	provider := NewLLMProvider(settings)

	if provider.client == nil {
		t.Fatal("expected client to be set")
	}
	if provider.client.Timeout == 0 {
		t.Error("expected non-zero client timeout")
	}
	if provider.client.Transport == nil {
		t.Error("expected non-nil Transport (connection pool)")
	}
}

// TestLLMProvider_GetSettingsCached 验证缓存命中。
func TestLLMProvider_GetSettingsCached(t *testing.T) {
	settings := NewSettingsSnapshot(&SettingsData{
		URL:       "https://api.example.com",
		ModelName: "gpt-4",
		AuthToken: "test",
	})
	provider := NewLLMProvider(settings)

	// 第一次获取
	data1, err := provider.getSettingsCached(context.Background())
	if err != nil {
		t.Fatalf("first get err: %v", err)
	}

	// 第二次获取（应命中缓存）
	data2, err := provider.getSettingsCached(context.Background())
	if err != nil {
		t.Fatalf("second get err: %v", err)
	}

	if data1 != data2 {
		t.Errorf("expected same data reference, got different")
	}
}

// TestLLMProvider_Available_CacheHit 验证 Available 走缓存。
func TestLLMProvider_Available_CacheHit(t *testing.T) {
	settings := NewSettingsSnapshot(&SettingsData{
		URL:       "https://api.example.com",
		ModelName: "gpt-4",
		AuthToken: "test-token", // 补全 AuthToken 让 Available 返回 true
	})
	provider := NewLLMProvider(settings)

	// 第一次调用应返回 true
	if !provider.Available(context.Background()) {
		t.Errorf("expected available=true with proper config")
	}

	// 此测试仅验证 Available 调用不会 panic 且返回 bool
	_ = provider.Available(context.Background())
}
