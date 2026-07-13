package ocr

import (
	"context"
	"os/exec"
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
	// 无规则源注入时，返回不匹配（不再是 stage-a 占位的恒 true）。
	got, err := rc.RulesCheck(context.Background(), "/some/path")
	if err != nil {
		t.Fatalf("RulesCheck err: %v", err)
	}
	if got.Applicable {
		t.Errorf("expected applicable=false when no rules configured")
	}
}

func TestViewer_Lifecycle(t *testing.T) {
	// 仅当 ocr 二进制可用时才跑真实生命周期测试（避免 CI 环境缺失二进制）。
	if _, err := exec.LookPath("ocr"); err != nil {
		t.Skip("ocr binary not on PATH; skipping viewer lifecycle test")
	}

	// 用一个随机端口避免与已运行的 viewer 冲突。
	addr := "localhost:15483"
	resolver := NewConfigResolver(NewSettingsSnapshot(&SettingsData{Binary: "ocr"}), Defaults(), nil)
	v := NewViewer(NewSettingsSnapshot(&SettingsData{Binary: "ocr"}), resolver)

	status, err := v.ViewerStatus(context.Background())
	if err != nil {
		t.Fatalf("ViewerStatus err: %v", err)
	}
	if status.Running {
		t.Fatalf("expected running=false initially, got %+v", status)
	}

	status, err = v.StartViewer(context.Background(), addr)
	if err != nil {
		t.Fatalf("StartViewer err: %v", err)
	}
	if !status.Running || !strings.Contains(status.Addr, "15483") {
		t.Fatalf("expected running=true addr contains 15483, got %+v", status)
	}

	// 确认 ViewerStatus 反映运行中状态。
	status, err = v.ViewerStatus(context.Background())
	if err != nil {
		t.Fatalf("ViewerStatus err: %v", err)
	}
	if !status.Running {
		t.Fatalf("expected running=true after start, got %+v", status)
	}

	status, err = v.StopViewer(context.Background())
	if err != nil {
		t.Fatalf("StopViewer err: %v", err)
	}
	if status.Running {
		t.Fatalf("expected running=false after stop, got %+v", status)
	}
}

func TestViewer_StartMissingBinary(t *testing.T) {
	resolver := NewConfigResolver(NewSettingsSnapshot(&SettingsData{Binary: "/nonexistent/ocr-binary"}), Defaults(), nil)
	v := NewViewer(NewSettingsSnapshot(&SettingsData{Binary: "/nonexistent/ocr-binary"}), resolver)

	status, err := v.StartViewer(context.Background(), "localhost:15484")
	if err == nil {
		t.Fatalf("expected error when binary is missing")
	}
	if status == nil || status.Running {
		t.Fatalf("expected running=false on missing binary, got %+v", status)
	}
}

func TestViewer_DefaultAddr(t *testing.T) {
	if _, err := exec.LookPath("ocr"); err != nil {
		t.Skip("ocr binary not on PATH; skipping viewer default addr test")
	}

	resolver := NewConfigResolver(NewSettingsSnapshot(&SettingsData{Binary: "ocr"}), Defaults(), nil)
	v := NewViewer(NewSettingsSnapshot(&SettingsData{Binary: "ocr"}), resolver)

	// 空 addr 应回退到 localhost:5483。用 StopViewer 兜底清理。
	status, err := v.StartViewer(context.Background(), "")
	if err != nil {
		t.Fatalf("StartViewer err: %v", err)
	}
	if !strings.Contains(status.Addr, "5483") {
		t.Fatalf("expected default addr contains 5483, got %s", status.Addr)
	}
	_, _ = v.StopViewer(context.Background())
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
