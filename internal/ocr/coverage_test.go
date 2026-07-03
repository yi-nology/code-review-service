package ocr

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yi-nology/code-review-service/internal/domain/ocrsettings"
)

// ===================== MetricsProvider 测试 =====================

func TestMetricsSnapshot(t *testing.T) {
	m := &Metrics{}
	m.Calls.Add(10)
	m.Successes.Add(8)
	m.Failures.Add(2)
	m.InputTokens.Add(1000)
	m.OutputTokens.Add(2000)
	m.TotalLatency.Add(int64(5 * time.Second))

	snap := m.Snapshot()
	if snap.Calls != 10 {
		t.Errorf("expected Calls=10, got %d", snap.Calls)
	}
	if snap.Successes != 8 {
		t.Errorf("expected Successes=8, got %d", snap.Successes)
	}
	if snap.Failures != 2 {
		t.Errorf("expected Failures=2, got %d", snap.Failures)
	}
	if snap.FailureRate != 0.2 {
		t.Errorf("expected FailureRate=0.2, got %f", snap.FailureRate)
	}
	if snap.AvgLatencyMs <= 0 {
		t.Errorf("expected positive AvgLatencyMs, got %f", snap.AvgLatencyMs)
	}
}

func TestMetricsSnapshot_ZeroCalls(t *testing.T) {
	m := &Metrics{}
	snap := m.Snapshot()
	if snap.Calls != 0 || snap.AvgLatencyMs != 0 || snap.FailureRate != 0 {
		t.Errorf("expected all zero for empty metrics")
	}
}

func TestNewMetricsProvider(t *testing.T) {
	mp, m := NewMetricsProvider(NewNoopProvider())
	if mp == nil || m == nil {
		t.Fatal("expected non-nil MetricsProvider and Metrics")
	}
}

func TestNewMetricsProvider_NilInner(t *testing.T) {
	mp, _ := NewMetricsProvider(nil)
	if mp == nil {
		t.Fatal("expected non-nil even with nil inner")
	}
	// nil inner should default to NoopProvider
	if mp.Available(context.Background()) {
		t.Errorf("expected Available=false for NoopProvider")
	}
}

func TestMetricsProvider_Available_True(t *testing.T) {
	settings := NewSettingsSnapshot(&SettingsData{
		URL:       "https://api.example.com",
		ModelName: "gpt-4",
	})
	inner := NewLLMProvider(settings)
	mp, m := NewMetricsProvider(inner)

	if !mp.Available(context.Background()) {
		t.Errorf("expected Available=true")
	}
	if m.Unavailable.Load() != 0 {
		t.Errorf("expected Unavailable=0 for available provider")
	}
}

func TestMetricsProvider_Available_False(t *testing.T) {
	mp, m := NewMetricsProvider(NewNoopProvider())

	if mp.Available(context.Background()) {
		t.Errorf("expected Available=false for NoopProvider")
	}
	if m.Unavailable.Load() != 1 {
		t.Errorf("expected Unavailable=1, got %d", m.Unavailable.Load())
	}
}

func TestMetricsProvider_Complete_Error(t *testing.T) {
	mp, m := NewMetricsProvider(NewNoopProvider())
	_, err := mp.Complete(context.Background(), CompleteRequest{})
	if err == nil {
		t.Errorf("expected error from NoopProvider")
	}
	if m.Failures.Load() != 1 {
		t.Errorf("expected Failures=1, got %d", m.Failures.Load())
	}
	if m.Calls.Load() != 1 {
		t.Errorf("expected Calls=1, got %d", m.Calls.Load())
	}
}

// ===================== Runner 错误类型测试 =====================

func TestTimeoutError(t *testing.T) {
	err := &TimeoutError{Timeout: 5 * time.Minute}
	if !strings.Contains(err.Error(), "超时") {
		t.Errorf("expected error to contain '超时', got %q", err.Error())
	}
	if !IsTimeout(err) {
		t.Errorf("expected IsTimeout=true")
	}
	if IsBinaryMissing(err) {
		t.Errorf("expected IsBinaryMissing=false for TimeoutError")
	}
}

func TestExitError(t *testing.T) {
	inner := errors.New("exit status 1")
	err := &ExitError{Stderr: "some stderr output", Err: inner}
	if !strings.Contains(err.Error(), "some stderr output") {
		t.Errorf("expected error to contain stderr, got %q", err.Error())
	}
	if !IsExit(err) {
		t.Errorf("expected IsExit=true")
	}
	if err.Unwrap() != inner {
		t.Errorf("expected Unwrap to return inner error")
	}
}

func TestExitError_EmptyStderr(t *testing.T) {
	inner := errors.New("exit status 1")
	err := &ExitError{Err: inner}
	if !strings.Contains(err.Error(), "exit status 1") {
		t.Errorf("expected error to contain inner error message")
	}
}

func TestBinaryMissingError_Unwrap(t *testing.T) {
	inner := errors.New("not found")
	err := &BinaryMissingError{Binary: "ocr", Err: inner}
	if err.Unwrap() != inner {
		t.Errorf("expected Unwrap to return inner error")
	}
}

// ===================== Runner 方法测试 =====================

func TestRunner_Refresh(t *testing.T) {
	r := NewRunner(Defaults().ToConfig())
	cfg2 := Config{Binary: "updated"}
	r.Refresh(cfg2)
	if r.Cfg().Binary != "updated" {
		t.Errorf("expected Binary=updated after Refresh")
	}
}

func TestRunner_Previewer(t *testing.T) {
	r := NewRunner(Defaults().ToConfig())
	if r.Previewer() == nil {
		t.Errorf("expected non-nil Previewer")
	}
}

func TestRunner_Scanner(t *testing.T) {
	r := NewRunner(Defaults().ToConfig())
	if r.Scanner() == nil {
		t.Errorf("expected non-nil Scanner")
	}
}

// ===================== Registry getter 测试 =====================

func TestRegistry_Resolver(t *testing.T) {
	resolver := NewConfigResolver(NewSettingsSnapshot(&SettingsData{}), Defaults(), nil)
	r := New(nil, Config{}, resolver, &stubTester{}, &stubLister{}, &stubChecker{}, &stubVersioner{}, &stubViewer{}, &stubUpgrader{}, &stubLLM{})
	if r.Resolver() != resolver {
		t.Errorf("expected Resolver to return same instance")
	}
}

func TestRegistry_Settings(t *testing.T) {
	// stubRepository 返回 nil，跳过此测试（Settings 需要 non-nil repos）
	// 实际 Settings 通过 SettingsAdapter 测试
	t.Skip("requires non-nil RepoContainer")
}

// ===================== resolveBinaryFrom 测试 =====================

func TestResolveBinaryFrom_Settings(t *testing.T) {
	s := &ocrsettings.Settings{Binary: "custom-ocr"}
	got := resolveBinaryFrom(s, Fallback{Binary: "fallback-ocr"})
	if got != "custom-ocr" {
		t.Errorf("expected custom-ocr, got %q", got)
	}
}

func TestResolveBinaryFrom_NilSettings(t *testing.T) {
	got := resolveBinaryFrom(nil, Fallback{Binary: "fallback-ocr"})
	if got != "fallback-ocr" {
		t.Errorf("expected fallback-ocr, got %q", got)
	}
}

// ===================== ensureOCRLanguage 测试 =====================

func TestEnsureOCRLanguage(t *testing.T) {
	// 写入临时配置文件
	home, _ := os.UserHomeDir()
	dir := filepath.Join(home, ".opencodereview")
	_ = os.MkdirAll(dir, 0o755)
	path := filepath.Join(dir, "config.json")
	_ = os.WriteFile(path, []byte(`{"language":"test-lang"}`), 0o644)

	// 调用 ensureOCRLanguage
	ensureOCRLanguage("ocr", "new-lang")

	// 验证文件被更新
	data, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("cannot read config file: %v", err)
	}
	if !strings.Contains(string(data), "new-lang") {
		t.Errorf("expected config to contain new-lang")
	}
}

// ===================== Config.AppendEnv 测试 =====================

func TestConfig_AppendEnv_Anthropic(t *testing.T) {
	cfg := Config{
		URL:          "https://api.anthropic.com",
		Model:        "claude-4",
		AuthToken:    "secret-key",
		UseAnthropic: true,
		Language:     "English",
	}
	env := cfg.AppendEnv([]string{})
	found := false
	for _, e := range env {
		if e == "OCR_USE_ANTHROPIC=true" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected OCR_USE_ANTHROPIC=true in env")
	}
}

func TestConfig_AppendEnv_Telemetry(t *testing.T) {
	cfg := Config{
		TelemetryEnabled:      true,
		TelemetryExporter:      "otlp",
		TelemetryOTLPEndpoint:  "http://localhost:4317",
		TelemetryContentLogging: true,
	}
	env := cfg.AppendEnv([]string{})
	expectations := []string{
		"OCR_ENABLE_TELEMETRY=true",
		"OCR_TELEMETRY_EXPORTER=otlp",
		"OCR_TELEMETRY_OTLP_ENDPOINT=http://localhost:4317",
		"OCR_CONTENT_LOGGING=true",
	}
	for _, exp := range expectations {
		found := false
		for _, e := range env {
			if e == exp {
				found = true
			}
		}
		if !found {
			t.Errorf("expected %s in env", exp)
		}
	}
}

// ===================== Command 公共入口测试 =====================

func TestInternalRun(t *testing.T) {
	res, err := InternalRun(context.Background(), CommandSpec{
		Binary: "echo",
		Args:   []string{"hello"},
	})
	if err != nil {
		t.Fatalf("InternalRun err: %v", err)
	}
	if !strings.Contains(string(res.Stdout), "hello") {
		t.Errorf("expected stdout to contain hello")
	}
}

func TestRun_Simplified(t *testing.T) {
	stdout, _, err := Run(context.Background(), "echo", "world")
	if err != nil {
		t.Fatalf("Run err: %v", err)
	}
	if !strings.Contains(string(stdout), "world") {
		t.Errorf("expected stdout to contain world")
	}
}

// ===================== Client 测试 =====================

func TestClient_Cfg(t *testing.T) {
	cfg := Defaults().ToConfig()
	cfg.Binary = "test-binary"
	c := NewClient(cfg)
	if c.Cfg().Binary != "test-binary" {
		t.Errorf("expected test-binary")
	}
}

func TestClient_WasRefreshed(t *testing.T) {
	c := NewClient(Defaults().ToConfig())
	cfg2 := Config{Binary: "refreshed"}
	c.Refresh(cfg2)
	if !c.WasRefreshed() {
		t.Errorf("expected WasRefreshed=true")
	}
	if c.Cfg().Binary != "refreshed" {
		t.Errorf("expected refreshed")
	}
}
