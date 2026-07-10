package postgres

import "testing"

func TestOCRSettingsRepoDefaultSettingsUsesConfiguredDefaults(t *testing.T) {
	repo := NewOCRSettingsRepo(nil, OCRSettingsDefaults{
		Enabled:         true,
		Binary:          " /usr/local/bin/ocr ",
		ContextWindow:   128000,
		MaxOutputTokens: 8192,
	})

	got := repo.defaultSettings()

	if !got.Enabled {
		t.Fatalf("expected default OCR enabled")
	}
	if got.Binary != "/usr/local/bin/ocr" {
		t.Fatalf("Binary = %q, want trimmed configured binary", got.Binary)
	}
	if got.ContextWindow != 128000 {
		t.Fatalf("ContextWindow = %d, want 128000", got.ContextWindow)
	}
	if got.MaxOutputTokens != 8192 {
		t.Fatalf("MaxOutputTokens = %d, want 8192", got.MaxOutputTokens)
	}
}

func TestOCRSettingsRepoDefaultSettingsKeepsBuiltInFallbacks(t *testing.T) {
	repo := NewOCRSettingsRepo(nil)

	got := repo.defaultSettings()

	if got.Enabled {
		t.Fatalf("expected built-in OCR default to be disabled")
	}
	if got.Binary != "ocr" {
		t.Fatalf("Binary = %q, want built-in ocr fallback", got.Binary)
	}
	if got.Audience != "agent" {
		t.Fatalf("Audience = %q, want agent", got.Audience)
	}
	if got.DefaultSeverity != "major" {
		t.Fatalf("DefaultSeverity = %q, want major", got.DefaultSeverity)
	}
	if got.Language != "Chinese" {
		t.Fatalf("Language = %q, want Chinese", got.Language)
	}
	if got.Concurrency != 2 {
		t.Fatalf("Concurrency = %d, want 2", got.Concurrency)
	}
	if got.MaxGitProcs != 4 {
		t.Fatalf("MaxGitProcs = %d, want 4", got.MaxGitProcs)
	}
	if got.TimeoutMinutes != 10 {
		t.Fatalf("TimeoutMinutes = %d, want 10", got.TimeoutMinutes)
	}
	if got.ReasoningMode != "light" {
		t.Fatalf("ReasoningMode = %q, want light", got.ReasoningMode)
	}
}
