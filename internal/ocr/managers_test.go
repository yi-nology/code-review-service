package ocr

import (
	"context"
	"strings"
	"testing"

	"github.com/yi-nology/code-review-service/internal/service/review/domain"
)

func TestTester_TestLLM(t *testing.T) {
	resolver := NewConfigResolver(NewSettingsSnapshot(&SettingsData{Binary: "ocr"}), Defaults(), nil)
	tester := NewTester(NewSettingsSnapshot(&SettingsData{Binary: "ocr"}), resolver)
	res, err := tester.TestLLM(context.Background())
	if err != nil {
		t.Fatalf("TestLLM err: %v", err)
	}
	if res == nil {
		t.Fatalf("expected non-nil result")
	}
}

func TestVersioner_OCRVersion(t *testing.T) {
	resolver := NewConfigResolver(NewSettingsSnapshot(&SettingsData{Binary: "ocr"}), Defaults(), nil)
	v := NewVersioner(NewSettingsSnapshot(&SettingsData{Binary: "ocr"}), resolver)
	got, err := v.OCRVersion(context.Background())
	if err != nil {
		t.Fatalf("OCRVersion err: %v", err)
	}
	if got.Version == "" {
		t.Errorf("expected non-empty version")
	}
}

func TestProviderLister_List(t *testing.T) {
	resolver := NewConfigResolver(NewSettingsSnapshot(&SettingsData{Binary: "ocr"}), Defaults(), nil)
	l := NewProviderLister(NewSettingsSnapshot(&SettingsData{Binary: "ocr"}), resolver)
	providers, err := l.ListLLMProviders(context.Background())
	if err != nil {
		t.Fatalf("ListLLMProviders err: %v", err)
	}
	if len(providers) == 0 {
		t.Errorf("expected at least one provider")
	}
}

func TestRulesChecker_RulesCheck(t *testing.T) {
	resolver := NewConfigResolver(NewSettingsSnapshot(&SettingsData{Binary: "ocr"}), Defaults(), nil)
	rc := NewRulesChecker(NewSettingsSnapshot(&SettingsData{Binary: "ocr"}), nil, resolver)
	got, err := rc.RulesCheck(context.Background(), "/some/path")
	if err != nil {
		t.Fatalf("RulesCheck err: %v", err)
	}
	if !got.Applicable {
		t.Errorf("expected applicable=true")
	}
}

func TestViewer_Lifecycle(t *testing.T) {
	resolver := NewConfigResolver(NewSettingsSnapshot(&SettingsData{Binary: "ocr"}), Defaults(), nil)
	v := NewViewer(NewSettingsSnapshot(&SettingsData{Binary: "ocr"}), resolver)

	status, err := v.ViewerStatus(context.Background())
	if err != nil {
		t.Fatalf("ViewerStatus err: %v", err)
	}
	if status.Running {
		t.Errorf("expected running=false initially")
	}

	status, err = v.StartViewer(context.Background(), "localhost:5483")
	if err != nil {
		t.Fatalf("StartViewer err: %v", err)
	}
	if !status.Running || !strings.Contains(status.Addr, "5483") {
		t.Errorf("expected running=true addr=5483, got %+v", status)
	}

	status, err = v.StopViewer(context.Background())
	if err != nil {
		t.Fatalf("StopViewer err: %v", err)
	}
	if status.Running {
		t.Errorf("expected running=false after stop")
	}
}

func TestUpgrader_CheckAndUpgrade(t *testing.T) {
	resolver := NewConfigResolver(NewSettingsSnapshot(&SettingsData{Binary: "ocr"}), Defaults(), nil)
	v := NewVersioner(NewSettingsSnapshot(&SettingsData{Binary: "ocr"}), resolver)
	u := NewUpgrader(NewSettingsSnapshot(&SettingsData{Binary: "ocr"}), resolver, v, "/tmp/ocr", "alibaba/open-code-review")

	info, err := u.CheckUpdate(context.Background())
	if err != nil {
		t.Fatalf("CheckUpdate err: %v", err)
	}
	if info.LatestVersion == "" {
		t.Errorf("expected non-empty LatestVersion")
	}

	status, err := u.Upgrade(context.Background(), "v0.1.0")
	if err != nil {
		t.Fatalf("Upgrade err: %v", err)
	}
	if status.State != domain.UpgradeIdle {
		t.Errorf("expected state=idle, got %v", status.State)
	}
}

func TestConfigResolver_Resolve(t *testing.T) {
	resolver := NewConfigResolver(
		NewSettingsSnapshot(&SettingsData{
			Binary:          "from-settings",
			URL:             "https://settings.example.com",
			DefaultSeverity: "minor",
		}),
		Defaults(),
		nil,
	)

	cfg, err := resolver.Resolve(context.Background(), OCROverrides{})
	if err != nil {
		t.Fatalf("Resolve err: %v", err)
	}
	if cfg.Binary != "from-settings" {
		t.Errorf("expected from-settings, got %q", cfg.Binary)
	}
	if cfg.URL != "https://settings.example.com" {
		t.Errorf("expected settings URL, got %q", cfg.URL)
	}
}

func TestFactory_CreateRunner(t *testing.T) {
	runner := NewRunner(Defaults().ToConfig())
	f := NewFactory(Defaults().ToConfig(), runner)

	got := f.CreateRunner("agent", `{"rules": []}`)
	if got == nil {
		t.Fatalf("CreateRunner returned nil")
	}
	cfg := got.Cfg()
	if cfg.Background != "agent" {
		t.Errorf("expected Background=agent, got %q", cfg.Background)
	}
}

func TestResolveBinaryFrom_Fallback(t *testing.T) {
	got := resolveBinaryFrom(nil, Defaults())
	if got != "ocr" {
		t.Errorf("expected default 'ocr', got %q", got)
	}
}
