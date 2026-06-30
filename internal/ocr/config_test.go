package ocr

import (
	"context"
	"testing"
)

func TestBuildConfig_PriorityOrder(t *testing.T) {
	settings := &SettingsData{
		Binary:          "from-settings",
		URL:             "https://settings.example.com",
		ModelName:       "settings-model",
		DefaultSeverity: "major",
		Language:        "go",
		AuthToken:       "settings-token",
	}
	overrides := OCROverrides{
		URL:    "https://override.example.com",
		Model:  "override-model",
	}
	fallback := Fallback{
		Binary:          "from-fallback",
		Audience:        "human",
		DefaultSeverity: "minor",
	}

	cfg := BuildConfig(settings, overrides, fallback, nil)

	if cfg.Binary != "from-settings" {
		t.Errorf("expected Binary from settings, got %q", cfg.Binary)
	}
	if cfg.URL != "https://override.example.com" {
		t.Errorf("expected URL from override, got %q", cfg.URL)
	}
	if cfg.Model != "override-model" {
		t.Errorf("expected Model from override, got %q", cfg.Model)
	}
	if cfg.AuthToken != "settings-token" {
		t.Errorf("expected AuthToken from settings (no override), got %q", cfg.AuthToken)
	}
	if cfg.Audience != "human" {
		t.Errorf("expected Audience from fallback, got %q", cfg.Audience)
	}
}

func TestBuildConfig_DefaultsWhenNothing(t *testing.T) {
	cfg := BuildConfig(&SettingsData{}, OCROverrides{}, Defaults(), nil)

	if cfg.Binary != "ocr" {
		t.Errorf("expected default Binary=ocr, got %q", cfg.Binary)
	}
	if cfg.Audience != "human" {
		t.Errorf("expected default Audience=human, got %q", cfg.Audience)
	}
	if cfg.DefaultSeverity != "major" {
		t.Errorf("expected default DefaultSeverity=major, got %q", cfg.DefaultSeverity)
	}
	if cfg.Concurrency != 2 {
		t.Errorf("expected default Concurrency=2, got %d", cfg.Concurrency)
	}
	if cfg.TimeoutMinutes != 10 {
		t.Errorf("expected default TimeoutMinutes=10, got %d", cfg.TimeoutMinutes)
	}
	if cfg.MaxGitProcs != 4 {
		t.Errorf("expected default MaxGitProcs=4, got %d", cfg.MaxGitProcs)
	}
}

func TestBuildConfig_OverrideTakesPrecedence(t *testing.T) {
	settings := &SettingsData{AuthToken: "settings-token"}
	overrides := OCROverrides{AuthToken: "override-token"}

	cfg := BuildConfig(settings, overrides, Defaults(), nil)
	if cfg.AuthToken != "override-token" {
		t.Errorf("expected AuthToken from override, got %q", cfg.AuthToken)
	}
}

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{"valid", Config{Binary: "ocr"}, false},
		{"empty binary", Config{}, true},
		{"timeout too large", Config{Binary: "ocr", TimeoutMinutes: 200}, true},
		{"concurrency too large", Config{Binary: "ocr", Concurrency: 100}, true},
		{"boundary", Config{Binary: "ocr", TimeoutMinutes: 120, Concurrency: 32}, false},
	}
	for _, tt := range tests {
		err := tt.cfg.Validate()
		if (err != nil) != tt.wantErr {
			t.Errorf("%s: Validate() err = %v, wantErr = %v", tt.name, err, tt.wantErr)
		}
	}
}

func TestConfig_AppendEnv(t *testing.T) {
	cfg := Config{
		URL:        "https://api.example.com",
		Model:      "gpt-4",
		AuthToken:  "secret",
		Language:   "go",
		UseAnthropic: true,
		ContextWindow: 100000,
	}
	env := cfg.AppendEnv([]string{"PATH=/usr/bin"})
	want := []string{
		"PATH=/usr/bin",
		"OCR_LLM_URL=https://api.example.com",
		"OCR_LLM_MODEL=gpt-4",
		"OCR_LLM_TOKEN=secret",
		"OCR_USE_ANTHROPIC=true",
		"OCR_LANGUAGE=go",
		"OCR_CONTEXT_WINDOW=100000",
	}
	for i, w := range want {
		if i >= len(env) || env[i] != w {
			t.Errorf("env[%d] = %q, want %q (full: %v)", i, env[i], w, env)
		}
	}
}

func TestSettingsSnapshot_GetAndSave(t *testing.T) {
	snap := NewSettingsSnapshot(&SettingsData{Binary: "ocr", URL: "https://x"})
	got, err := snap.Get(context.Background())
	if err != nil {
		t.Fatalf("Get err: %v", err)
	}
	if got.Binary != "ocr" {
		t.Errorf("expected Binary=ocr, got %q", got.Binary)
	}
	if err := snap.Save(context.Background(), &SettingsData{}); err != nil {
		t.Errorf("Save should be no-op for snapshot, got err: %v", err)
	}
}

func TestSettingsSnapshot_GetNilReturnsError(t *testing.T) {
	snap := NewSettingsSnapshot(nil)
	if _, err := snap.Get(context.Background()); err == nil {
		t.Errorf("expected err for nil snapshot, got nil")
	}
}

func TestFirstNonBlank(t *testing.T) {
	if firstNonBlank("", "  ", "hello", "world") != "hello" {
		t.Errorf("expected hello")
	}
	if firstNonBlank("", "") != "" {
		t.Errorf("expected empty")
	}
	if firstNonBlank("first") != "first" {
		t.Errorf("expected first")
	}
}

func TestFirstPositive(t *testing.T) {
	if firstPositive(0, -1, 5, 10) != 5 {
		t.Errorf("expected 5")
	}
	if firstPositive(0, 0) != 0 {
		t.Errorf("expected 0 (no positive)")
	}
	if firstPositive(3) != 3 {
		t.Errorf("expected 3")
	}
}