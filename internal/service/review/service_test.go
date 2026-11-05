package review_test

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/yi-nology/code-review-service/internal/domain/review"
	"github.com/yi-nology/code-review-service/internal/service/policy"
	"github.com/yi-nology/code-review-service/internal/service/review"
)

func TestScanRecordsRawOCRInput(t *testing.T) {
	workspaceDir := t.TempDir()

	repo := &mockReviewRepo{}
	scanRunner := &stubScanRunner{result: &domain.ScanResult{
		Result: &domain.Result{Summary: "scan ok", OCR: domain.OCRMetadata{Status: "success", FilesReviewed: 1}},
	}}

	svc := review.NewService(review.ServiceConfig{
		ReviewRepo:      repo,
		OCRSettingsRepo: &mockOCRSettingsRepo{},
		ProjectRepo:     &mockProjectPolicyRepo{},
		Policy:          policy.NewEngine(policy.Config{FailOn: "critical"}),
		Providers: newTestProviders(testProvider{
			name: "gitea",
			prepare: func(_ context.Context, req domain.Request) (domain.Workspace, error) {
				return domain.Workspace{Repo: req.Repo, BaseSHA: req.BaseSHA, HeadSHA: req.HeadSHA, Dir: workspaceDir}, nil
			},
		}),
		ScanRunner: scanRunner,
	})

	result, err := svc.Scan(context.Background(), domain.ScanRequest{
		Provider: "gitea",
		Repo:     "acme/app",
		HeadSHA:  "f6e5d4c3b2a1f6e5d4c3b2a1f6e5d4c3b2a1f6e5",
	})
	if err != nil {
		t.Fatalf("Scan returned error: %v", err)
	}

	deadline := time.Now().Add(5 * time.Second)
	var saved *domain.Record
	for time.Now().Before(deadline) {
		for _, r := range repo.snapshot() {
			if r.ID == result.ReviewID {
				saved = r
			}
		}
		if saved != nil && saved.Status == "success" {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if saved == nil {
		t.Fatal("expected scan review to be persisted")
	}

	if len(saved.RawOCRInput) == 0 {
		t.Fatal("expected raw_ocr_input to be recorded for scan, got empty")
	}
	var input domain.OCRInput
	if err := json.Unmarshal(saved.RawOCRInput, &input); err != nil {
		t.Fatalf("failed to decode raw_ocr_input: %v", err)
	}
	if input.Workspace.Dir != workspaceDir {
		t.Fatalf("expected workspace dir %q, got %q", workspaceDir, input.Workspace.Dir)
	}
	if input.StartedAt.IsZero() {
		t.Fatal("expected non-zero started_at in raw_ocr_input")
	}
	if _, err := os.Stat(workspaceDir); err == nil {
		_ = os.RemoveAll(workspaceDir)
	}
}

// TestScanAsync_RequestCtxCancellation 验证 PR-03 修复：
// 当 HTTP 请求 ctx 被取消时，scanAsync 启动的 goroutine 应当感知到 ctx.Done()。
// 旧实现 context.WithCancel(context.Background()) 与请求 lifecycle 完全脱钩，
// 只能通过显式 Service.Cancel() 取消；新实现派生自 ctx，请求断开即终止。
//
// 验证方式：scan runner 在 ctx.Done() 触发时记录一次信号；测试取消 ctx 后等待信号。
func TestScanAsync_RequestCtxCancellation(t *testing.T) {
	scanRunner := &ctxBlockingScanRunner{entered: make(chan struct{}, 1)}

	svc := review.NewService(review.ServiceConfig{
		ReviewRepo:      &mockReviewRepo{},
		OCRSettingsRepo: &mockOCRSettingsRepo{},
		ProjectRepo:     &mockProjectPolicyRepo{},
		Policy:          policy.NewEngine(policy.Config{FailOn: "critical"}),
		Providers: newTestProviders(testProvider{
			name: "gitea",
			prepare: func(_ context.Context, _ domain.Request) (domain.Workspace, error) {
				return domain.Workspace{Dir: t.TempDir()}, nil
			},
		}),
		Runner:     &countingRunner{},
		ScanRunner: scanRunner,
		Executor:   mockExecutor{runner: &countingRunner{}, repo: &mockReviewRepo{}},
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	scanReq := domain.ScanRequest{
		Provider: "gitea",
		Repo:     "owner/repo",
		HeadSHA:  "deadbeef",
		Batch:    "none",
	}

	result, err := svc.Scan(ctx, scanReq)
	if err != nil {
		t.Fatalf("Scan returned error: %v", err)
	}
	if result == nil || result.ReviewID == "" {
		t.Fatalf("expected review ID, got %+v", result)
	}

	// 等待 scan runner 进入阻塞态（已收到 ctx）
	select {
	case <-scanRunner.entered:
	case <-time.After(2 * time.Second):
		t.Fatalf("scan runner did not start within 2s")
	}

	// 取消请求 ctx
	cancel()

	// 等待 scan runner 收到 ctx.Done()
	select {
	case <-scanRunner.cancelled:
		// 成功
	case <-time.After(2 * time.Second):
		t.Fatalf("scan runner did not receive ctx cancellation within 2s (PR-03 broken?)")
	}
}

// ctxBlockingScanRunner 阻塞在 ctx.Done() 上以验证 cancellation。
type ctxBlockingScanRunner struct {
	entered   chan struct{}
	cancelled chan struct{}
}

func (r *ctxBlockingScanRunner) Scan(ctx context.Context, _ domain.Workspace, _ domain.ScanRequest) (*domain.ScanResult, error) {
	select {
	case r.entered <- struct{}{}:
	default:
	}
	if r.cancelled == nil {
		r.cancelled = make(chan struct{})
	}
	<-ctx.Done()
	close(r.cancelled)
	return nil, ctx.Err()
}

// TestMergeRequirementsIntoBackground 验证需求/spec 被正确合并进 OCR --background。
func TestMergeRequirementsIntoBackground(t *testing.T) {
	tests := []struct {
		name         string
		existing     string
		requirements []string
		want         string
	}{
		{
			name:         "empty existing + requirements",
			existing:     "",
			requirements: []string{"需求 A", "需求 B"},
			want:         "需求 A\n需求 B",
		},
		{
			name:         "existing + requirements appended",
			existing:     "项目背景",
			requirements: []string{"需求 A"},
			want:         "项目背景\n需求 A",
		},
		{
			name:         "no requirements keeps existing",
			existing:     "项目背景",
			requirements: nil,
			want:         "项目背景",
		},
		{
			name:         "blank entries trimmed and dropped",
			existing:     "  项目背景  ",
			requirements: []string{"  ", "需求 A", ""},
			want:         "项目背景\n需求 A",
		},
		{
			name:         "all empty produces empty",
			existing:     "   ",
			requirements: []string{"  ", ""},
			want:         "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := review.MergeRequirementsIntoBackground(tt.existing, tt.requirements)
			if got != tt.want {
				t.Errorf("MergeRequirementsIntoBackground(%q, %v) = %q, want %q", tt.existing, tt.requirements, got, tt.want)
			}
		})
	}
}
