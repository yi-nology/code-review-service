package executor

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/yi-nology/code-review-service/internal/repository"
	"github.com/yi-nology/code-review-service/internal/repository/repoerr"
	"github.com/yi-nology/code-review-service/internal/service/review/domain"

	ekitGorm "github.com/oekit/gormx" //lint:ignore SA1019 PageResponse 未废弃，上游包级别注释误标
)

// stubWriteback 是 domain.WritebackPublisher 的可控实现。
// 行为由 err 字段决定：nil 成功；非 nil 模拟写回平台失败。
type stubWriteback struct {
	err      error
	skipped  bool
	calls    int
	lastReq  domain.Request
	lastDiff string
}

func (s *stubWriteback) PublishWithDiff(_ context.Context, req domain.Request, _ domain.Result, diff string) (*domain.WritebackResult, error) {
	s.calls++
	s.lastReq = req
	s.lastDiff = diff
	if s.err != nil {
		return nil, s.err
	}
	return &domain.WritebackResult{Published: 1, Skipped: s.skipped}, nil
}

// noopReviewRepo 是一个不依赖 DB 的最小 repository.ReviewRepository 占位。
// 所有写方法返回 nil；读方法返回零值。
// publishWriteback 间接调用 appendHistory → UpdateInTransaction，
// 我们让它返回 nil（视为成功）以避免测试日志噪音。
type noopReviewRepo struct{}

func (noopReviewRepo) Create(context.Context, *domain.Record) error { return nil }
func (noopReviewRepo) Save(context.Context, *domain.Record) error    { return nil }
func (noopReviewRepo) GetByReviewID(context.Context, string) (*domain.Record, error) {
	return &domain.Record{}, nil
}
func (noopReviewRepo) GetByIdempotencyKey(context.Context, string) (*domain.Record, error) {
	return nil, repoerr.ErrNotFound
}
func (noopReviewRepo) GetActiveByProviderRepo(context.Context, string, string, string, string) (*domain.Record, error) {
	return nil, repoerr.ErrNotFound
}
func (noopReviewRepo) List(context.Context, domain.ListOptions) (*ekitGorm.PageResponse[domain.Record], error) {
	return &ekitGorm.PageResponse[domain.Record]{}, nil
}
func (noopReviewRepo) AggregateStats(context.Context, domain.ListOptions) (*domain.AggregateStats, error) {
	return &domain.AggregateStats{}, nil
}
func (noopReviewRepo) ProjectOCRMetrics(context.Context, domain.ProjectStatsOptions) (*domain.ProjectOCRMetrics, error) {
	return &domain.ProjectOCRMetrics{}, nil
}
func (noopReviewRepo) ProjectOCRTrend(context.Context, domain.ProjectStatsOptions) ([]domain.ProjectOCRTrendPoint, error) {
	return nil, nil
}
func (noopReviewRepo) ClaimQueued(context.Context, int) ([]domain.Record, error) {
	return nil, nil
}
func (noopReviewRepo) RequeueStale(context.Context, time.Time) (int64, error) { return 0, nil }
func (noopReviewRepo) UpdateInTransaction(context.Context, string, func(*domain.Record) error) error {
	return nil
}

var (
	_ repository.ReviewRepository = noopReviewRepo{}
)

// TestPublishWriteback_FailureSurfaces 验证 PR-01 修复点：
// 当底层 writeback.PublishWithDiff 返回 error 时，publishWriteback 必须把
// error 透传（不再吞错），且 summary.Status == failed。
func TestPublishWriteback_FailureSurfaces(t *testing.T) {
	svc := &Service{
		reviewRepo: noopReviewRepo{},
		writeback:  &stubWriteback{err: errors.New("github 401 unauthorized")},
	}

	req := domain.Request{Provider: "github", Repo: "o/r", PullRequestID: "1", CallbackMode: "comment"}
	summary, err := svc.publishWriteback(context.Background(), "rv_1", req, domain.Workspace{}, domain.Result{})

	if err == nil {
		t.Fatalf("expected error to surface, got nil")
	}
	if !strings.Contains(err.Error(), "github 401 unauthorized") {
		t.Fatalf("expected error to wrap original, got %v", err)
	}
	if summary.Status != domain.WritebackStatusFailed {
		t.Fatalf("expected status=failed, got %q", summary.Status)
	}
	if summary.Error == "" {
		t.Fatalf("expected summary.Error to be set, got empty")
	}
}

// TestPublishWriteback_Success 验证正常路径：summary.Status=ok, Published 数透传。
func TestPublishWriteback_Success(t *testing.T) {
	svc := &Service{
		reviewRepo: noopReviewRepo{},
		writeback:  &stubWriteback{},
	}

	req := domain.Request{Provider: "github", Repo: "o/r", PullRequestID: "1", CallbackMode: "comment"}
	summary, err := svc.publishWriteback(context.Background(), "rv_1", req, domain.Workspace{}, domain.Result{})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if summary.Status != domain.WritebackStatusOK {
		t.Fatalf("expected status=ok, got %q", summary.Status)
	}
	if summary.Published != 1 {
		t.Fatalf("expected published=1, got %d", summary.Published)
	}
}

// TestPublishWriteback_Disabled 验证无 writeback publisher 时返回 disabled。
func TestPublishWriteback_Disabled(t *testing.T) {
	svc := &Service{reviewRepo: noopReviewRepo{}, writeback: nil}

	summary, err := svc.publishWriteback(context.Background(), "rv_1", domain.Request{}, domain.Workspace{}, domain.Result{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if summary.Status != domain.WritebackStatusDisabled {
		t.Fatalf("expected status=disabled, got %q", summary.Status)
	}
}

// TestPublishWriteback_Skipped 验证无 PR/MR 且 callbackMode=comment 时被改写为 none 并跳过。
func TestPublishWriteback_Skipped(t *testing.T) {
	stub := &stubWriteback{}
	svc := &Service{reviewRepo: noopReviewRepo{}, writeback: stub}

	req := domain.Request{Provider: "github", Repo: "o/r", CallbackMode: "comment"}
	summary, err := svc.publishWriteback(context.Background(), "rv_1", req, domain.Workspace{}, domain.Result{})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if summary.Status != domain.WritebackStatusSkipped {
		t.Fatalf("expected status=skipped, got %q", summary.Status)
	}
	if stub.calls != 0 {
		t.Fatalf("expected publisher not to be called, got %d calls", stub.calls)
	}
}

// TestWritebackSummary_RoundTrip 验证 WritebackSummary 字段在 JSON 序列化中正确编解码。
// 目的：确保 API 响应里前端能正确读出 status / published / failed / skipped / error。
func TestWritebackSummary_RoundTrip(t *testing.T) {
	resp := domain.Response{
		ReviewID: "rv_1",
		Status:   domain.StatusSuccessWithPublishFailed,
		Writeback: domain.WritebackSummary{
			Status:    domain.WritebackStatusFailed,
			Published: 0,
			Failed:    3,
			Error:     "github 500",
		},
	}
	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(data), `"status":"success_with_publish_failed"`) {
		t.Fatalf("expected status in JSON, got %s", data)
	}
	if !strings.Contains(string(data), `"writeback":{`) {
		t.Fatalf("expected expected writeback summary in JSON, got %s", data)
	}
}
