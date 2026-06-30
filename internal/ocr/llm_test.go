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
	p := NewLLMProvider(NewSettingsSnapshot(&SettingsData{URL: "https://api.example.com"}))
	if !p.Available(context.Background()) {
		t.Errorf("expected Available=true when URL set")
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
	p := NewLLMProvider(NewSettingsSnapshot(&SettingsData{
		URL:        "https://api.example.com",
		AuthToken:  "secret",
		ModelName:  "gpt-4",
		ExtraBody:  "{}",
	}))
	resp, err := p.Complete(context.Background(), LLMRequest{UserPrompt: "hi"})
	if err != nil {
		t.Fatalf("expected nil err for placeholder, got %v", err)
	}
	if resp.Content != "stage-a-placeholder" {
		t.Errorf("expected stage-a-placeholder, got %q", resp.Content)
	}
}

func TestLLMProviderAdapter_RoundTrip(t *testing.T) {
	p := NewLLMProvider(NewSettingsSnapshot(&SettingsData{URL: "https://x"}))
	adapter := &LLMProviderAdapter{inner: p}
	if !adapter.Available(context.Background()) {
		t.Errorf("expected available")
	}
	resp, err := adapter.Complete(context.Background(), CompleteRequest{UserPrompt: "hi"})
	if err != nil {
		t.Fatalf("Complete err: %v", err)
	}
	if resp.Content != "stage-a-placeholder" {
		t.Errorf("expected placeholder, got %q", resp.Content)
	}
}

func TestErrLLMNotConfigured(t *testing.T) {
	if errLLMNotConfigured.Error() != "llm: not configured" {
		t.Errorf("unexpected message: %v", errLLMNotConfigured)
	}
}