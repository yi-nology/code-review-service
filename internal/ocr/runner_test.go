package ocr

import (
	"context"
	"errors"
	"testing"

	"github.com/yi-nology/code-review-service/internal/service/review/domain"
)

// stubLegacy 实现 legacyRunner 接口，用于测试 Runner 委托逻辑。
type stubLegacy struct {
	runOut     domain.Result
	runErr     error
	previewOut *domain.PreviewResult
	previewErr error
	scanOut    *domain.ScanResult
	scanErr    error

	gotCfg        Config
	invocations   int
}

func (s *stubLegacy) RunWithConfig(_ context.Context, _ domain.Workspace, cfg Config) (domain.Result, error) {
	s.gotCfg = cfg
	s.invocations++
	return s.runOut, s.runErr
}

func (s *stubLegacy) PreviewWithConfig(_ context.Context, _ domain.Workspace, cfg Config) (*domain.PreviewResult, error) {
	s.gotCfg = cfg
	s.invocations++
	return s.previewOut, s.previewErr
}

func (s *stubLegacy) ScanWithConfig(_ context.Context, _ domain.Workspace, req domain.ScanRequest, cfg Config) (*domain.ScanResult, error) {
	s.gotCfg = cfg
	s.invocations++
	return s.scanOut, s.scanErr
}

func TestRunner_DelegatesRunToLegacy(t *testing.T) {
	want := domain.Result{Summary: "ok"}
	legacy := &stubLegacy{runOut: want}
	cfg := Defaults().ToConfig()
	r := NewRunnerWithLegacy(cfg, legacy)

	got, err := r.Run(context.Background(), domain.Workspace{Dir: "/tmp"})
	if err != nil {
		t.Fatalf("Run err: %v", err)
	}
	if got.Summary != "ok" {
		t.Errorf("expected Summary=ok, got %q", got.Summary)
	}
	if legacy.invocations != 1 {
		t.Errorf("expected 1 invocation, got %d", legacy.invocations)
	}
}

func TestRunner_DelegatesPreviewAndScan(t *testing.T) {
	legacy := &stubLegacy{
		previewOut: &domain.PreviewResult{Files: []string{"a.go"}},
		scanOut:    &domain.ScanResult{},
	}
	cfg := Defaults().ToConfig()
	r := NewRunnerWithLegacy(cfg, legacy)

	pv, err := r.Preview(context.Background(), domain.Workspace{Dir: "/tmp"})
	if err != nil {
		t.Fatalf("Preview err: %v", err)
	}
	if len(pv.Files) != 1 || pv.Files[0] != "a.go" {
		t.Errorf("expected [a.go], got %v", pv.Files)
	}

	sc, err := r.Scan(context.Background(), domain.Workspace{Dir: "/tmp"}, domain.ScanRequest{Batch: "none"})
	if err != nil {
		t.Fatalf("Scan err: %v", err)
	}
	if sc == nil {
		t.Errorf("expected non-nil ScanResult")
	}
}

func TestRunner_NoLegacyReturnsError(t *testing.T) {
	r := NewRunner(Defaults().ToConfig())
	_, err := r.Run(context.Background(), domain.Workspace{Dir: "/tmp"})
	if err == nil {
		t.Fatalf("expected err when legacy is nil")
	}
}

func TestRunner_LegacyErrorPropagates(t *testing.T) {
	legacy := &stubLegacy{runErr: errors.New("boom")}
	r := NewRunnerWithLegacy(Defaults().ToConfig(), legacy)

	_, err := r.Run(context.Background(), domain.Workspace{Dir: "/tmp"})
	if err == nil || err.Error() != "boom" {
		t.Fatalf("expected boom, got %v", err)
	}
}

func TestRunner_RefreshReplacesCfg(t *testing.T) {
	legacy := &stubLegacy{}
	cfg1 := Defaults().ToConfig()
	cfg1.Binary = "first"
	r := NewRunnerWithLegacy(cfg1, legacy)
	_, _ = r.Run(context.Background(), domain.Workspace{Dir: "/tmp"})
	if legacy.gotCfg.Binary != "first" {
		t.Errorf("expected first, got %q", legacy.gotCfg.Binary)
	}

	cfg2 := Defaults().ToConfig()
	cfg2.Binary = "second"
	r.Refresh(cfg2)
	_, _ = r.Run(context.Background(), domain.Workspace{Dir: "/tmp"})
	if legacy.gotCfg.Binary != "second" {
		t.Errorf("expected second after Refresh, got %q", legacy.gotCfg.Binary)
	}
}