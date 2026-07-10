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

func TestLLMProviderAdapter_SatisfiesLLMProvider(t *testing.T) {
	// 阶段 A15 关键断言：*LLMProviderAdapter 必须实现 llm.Provider 接口。
	settings := NewSettingsSnapshot(&SettingsData{
		URL:       "https://api.example.com",
		ModelName: "gpt-4",
	})
	provider := NewLLMProvider(settings)
	adapter := WrapLLMProvider(provider)

	// 编译期检查：把 adapter 赋值给 llm.Provider 接口变量
	var p llmProviderInterface = adapter
	if p == nil {
		t.Fatalf("adapter should not be nil")
	}
}

// llmProviderInterface 是 llm.Provider 的内部 alias（避免 import llm 到 ocr 测试文件）。
type llmProviderInterface interface {
	Available(ctx context.Context) bool
	Complete(ctx context.Context, req LLMRequest) (LLMResponse, error)
}
