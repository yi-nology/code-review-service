package review_test

import (
	"context"
	"errors"
	"testing"

	"github.com/yi-nology/code-review-service/internal/domain/review"
	"github.com/yi-nology/code-review-service/internal/service/review"
)

// stubDelegator 实现 domain.Delegator，记录调用参数并返回预设结果。
type stubDelegator struct {
	previewResult *domain.DelegatePreviewResult
	ruleResult    *domain.DelegateRuleResult
	err           error
	// 记录最后一次调用的 workspace，供断言 paths/exclude 透传是否正确。
	lastWorkspace domain.Workspace
	lastPaths     []string
}

func (d *stubDelegator) DelegatePreview(_ context.Context, ws domain.Workspace) (*domain.DelegatePreviewResult, error) {
	d.lastWorkspace = ws
	if d.err != nil {
		return nil, d.err
	}
	return d.previewResult, nil
}

func (d *stubDelegator) DelegateRule(_ context.Context, ws domain.Workspace, paths []string) (*domain.DelegateRuleResult, error) {
	d.lastWorkspace = ws
	d.lastPaths = paths
	if d.err != nil {
		return nil, d.err
	}
	return d.ruleResult, nil
}

// TestDelegatePreview_NoDelegator 校验 Delegator 未注入时返回明确错误（OCR < v1.7.11）。
func TestDelegatePreview_NoDelegator(t *testing.T) {
	svc := review.NewService(review.ServiceConfig{
		Providers: newTestProviders(testProvider{
			name: "gitea",
			prepare: func(_ context.Context, _ domain.Request) (domain.Workspace, error) {
				return domain.Workspace{Dir: "/tmp"}, nil
			},
		}),
	})
	_, err := svc.DelegatePreview(context.Background(), domain.Request{Provider: "gitea", Repo: "acme/app", BaseSHA: "abc123def4", HeadSHA: "abc123def5"}, nil)
	if err == nil {
		t.Fatal("expected error when Delegator is nil")
	}
}

// TestDelegatePreview_PassesExcludeToWorkspace 校验 HTTP 层的 exclude 透传到 workspace.ScanExclude。
func TestDelegatePreview_PassesExcludeToWorkspace(t *testing.T) {
	d := &stubDelegator{
		previewResult: &domain.DelegatePreviewResult{Mode: "range", Files: []string{"a.go"}},
	}
	svc := review.NewService(review.ServiceConfig{
		Providers: newTestProviders(testProvider{
			name: "gitea",
			prepare: func(_ context.Context, req domain.Request) (domain.Workspace, error) {
				return domain.Workspace{Dir: "/tmp", Repo: req.Repo}, nil
			},
		}),
		Delegator: d,
	})
	got, err := svc.DelegatePreview(context.Background(),
		domain.Request{Provider: "gitea", Repo: "acme/app", BaseSHA: "abc123def4", HeadSHA: "abc123def5"},
		[]string{"vendor/**", "dist/**"},
	)
	if err != nil {
		t.Fatalf("DelegatePreview err: %v", err)
	}
	if got.Mode != "range" || len(got.Files) != 1 {
		t.Errorf("unexpected result: %+v", got)
	}
	if len(d.lastWorkspace.ScanExclude) != 2 ||
		d.lastWorkspace.ScanExclude[0] != "vendor/**" ||
		d.lastWorkspace.ScanExclude[1] != "dist/**" {
		t.Errorf("ScanExclude not propagated: %+v", d.lastWorkspace.ScanExclude)
	}
}

// TestDelegateRule_PassesPathsToDelegator 校验 paths 列表透传给 Delegator.DelegateRule。
func TestDelegateRule_PassesPathsToDelegator(t *testing.T) {
	d := &stubDelegator{
		ruleResult: &domain.DelegateRuleResult{RawOutput: "### Rule Group 1"},
	}
	svc := review.NewService(review.ServiceConfig{
		Providers: newTestProviders(testProvider{
			name: "gitea",
			prepare: func(_ context.Context, req domain.Request) (domain.Workspace, error) {
				return domain.Workspace{Dir: "/tmp", Repo: req.Repo}, nil
			},
		}),
		Delegator: d,
	})
	paths := []string{"main.go", "internal/agent/agent.go"}
	got, err := svc.DelegateRule(context.Background(),
		domain.Request{Provider: "gitea", Repo: "acme/app", BaseSHA: "abc123def4", HeadSHA: "abc123def5"},
		paths,
	)
	if err != nil {
		t.Fatalf("DelegateRule err: %v", err)
	}
	if got.RawOutput != "### Rule Group 1" {
		t.Errorf("unexpected RawOutput: %q", got.RawOutput)
	}
	if len(d.lastPaths) != 2 || d.lastPaths[0] != "main.go" {
		t.Errorf("paths not propagated: %+v", d.lastPaths)
	}
	// paths 也应写入 workspace.ScanPaths，供底层 runner 使用。
	if len(d.lastWorkspace.ScanPaths) != 2 {
		t.Errorf("ScanPaths not set: %+v", d.lastWorkspace.ScanPaths)
	}
}

// TestDelegatePreview_PropagatesError 校验底层错误被透传到调用方。
func TestDelegatePreview_PropagatesError(t *testing.T) {
	expectedErr := errors.New("ocr binary missing")
	d := &stubDelegator{err: expectedErr}
	svc := review.NewService(review.ServiceConfig{
		Providers: newTestProviders(testProvider{
			name: "gitea",
			prepare: func(_ context.Context, _ domain.Request) (domain.Workspace, error) {
				return domain.Workspace{Dir: "/tmp"}, nil
			},
		}),
		Delegator: d,
	})
	_, err := svc.DelegatePreview(context.Background(), domain.Request{Provider: "gitea", Repo: "acme/app", BaseSHA: "abc123def4", HeadSHA: "abc123def5"}, nil)
	if !errors.Is(err, expectedErr) {
		t.Errorf("expected err to propagate, got: %v", err)
	}
}
