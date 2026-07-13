package ocr

import (
	"context"
	"testing"
)

func TestLLMProvider_AvailableNoConfig(t *testing.T) {
	p := NewLLMProvider(NewSettingsSnapshot(&SettingsData{}))
	if p.Available(context.Background()) {
		t.Errorf("expected Available=false for empty settings")
	}
}

func TestLLMProvider_AvailableWithURL(t *testing.T) {
	p := NewLLMProvider(NewSettingsSnapshot(&SettingsData{
		URL:       "https://api.example.com",
		ModelName: "gpt-4",
	}))
	if !p.Available(context.Background()) {
		t.Errorf("expected Available=true when URL+Model set")
	}
}

func TestLLMProvider_CompleteNotConfigured(t *testing.T) {
	p := NewLLMProvider(NewSettingsSnapshot(&SettingsData{}))
	_, err := p.Complete(context.Background(), LLMRequest{UserPrompt: "hi"})
	if err == nil {
		t.Errorf("expected err for not configured")
	}
}

func TestLLMProvider_CompletePlaceholder(t *testing.T) {
	// 阶段 A11 完整化：不再 placeholder；该测试改为验证「未配置返回 err」。
	p := NewLLMProvider(NewSettingsSnapshot(&SettingsData{
		URL:       "https://api.example.com",
		ModelName: "gpt-4",
		AuthToken: "secret",
	}))
	// 由于 endpoint 是无效域名，调用应失败（连接错误），但不应该 panic。
	_, err := p.Complete(context.Background(), LLMRequest{UserPrompt: "hi"})
	if err == nil {
		t.Errorf("expected err from invalid endpoint")
	}
	// 错误必须包含某种描述
	if err.Error() == "" {
		t.Errorf("expected non-empty error message")
	}
}

func TestLLMProviderAdapter_RoundTrip(t *testing.T) {
	p := NewLLMProvider(NewSettingsSnapshot(&SettingsData{
		URL:       "https://x",
		ModelName: "gpt-4",
	}))
	adapter := WrapLLMProvider(p)
	if !adapter.Available(context.Background()) {
		t.Errorf("expected available")
	}
	// endpoint 是无效域名，调用必失败；但 Adapter 转发路径必须工作
	_, err := adapter.Complete(context.Background(), CompleteRequest{UserPrompt: "hi"})
	if err == nil {
		t.Errorf("expected err from invalid endpoint")
	}
}

func TestErrLLMNotConfigured(t *testing.T) {
	if errLLMNotConfigured.Error() != "llm: not configured" {
		t.Errorf("unexpected message: %v", errLLMNotConfigured)
	}
}
