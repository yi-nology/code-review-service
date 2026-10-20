package ocr

import (
	"context"
	"testing"

	"github.com/yi-nology/code-review-service/internal/domain/ocrsettings"
)

// stubSettingsRepo 是 OCRSettingsGetter 的 stub。
type stubSettingsRepo struct {
	data *ocrsettings.Settings
	err  error
}

func (s *stubSettingsRepo) Get(_ context.Context) (*ocrsettings.Settings, error) {
	return s.data, s.err
}

func TestSettingsAdapter_Get(t *testing.T) {
	repo := &stubSettingsRepo{
		data: &ocrsettings.Settings{
			Binary:    "ocr",
			URL:       "https://api.example.com",
			ModelName: "gpt-4",
			Language:  "go",
		},
	}
	a := NewSettingsAdapter(repo)
	data, err := a.Get(context.Background())
	if err != nil {
		t.Fatalf("Get err: %v", err)
	}
	if data.Binary != "ocr" {
		t.Errorf("expected Binary=ocr, got %q", data.Binary)
	}
	if data.URL != "https://api.example.com" {
		t.Errorf("expected URL set")
	}
	if data.Language != "go" {
		t.Errorf("expected Language=go, got %q", data.Language)
	}
}

func TestSettingsAdapter_GetError(t *testing.T) {
	repo := &stubSettingsRepo{err: context.DeadlineExceeded}
	a := NewSettingsAdapter(repo)
	_, err := a.Get(context.Background())
	if err == nil {
		t.Fatalf("expected err")
	}
}

func TestSettingsAdapter_GetNil(t *testing.T) {
	repo := &stubSettingsRepo{data: nil}
	a := NewSettingsAdapter(repo)
	data, err := a.Get(context.Background())
	if err != nil {
		t.Fatalf("Get err: %v", err)
	}
	if data != nil {
		t.Errorf("expected nil data for nil settings, got %+v", data)
	}
}

func TestConvertOCRSettings_NilInput(t *testing.T) {
	if got := convertOCRSettings(nil); got != nil {
		t.Errorf("expected nil for nil input, got %+v", got)
	}
}
