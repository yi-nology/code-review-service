package review

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/yi-nology/code-review-service/internal/repository/repoerr"
	"github.com/yi-nology/code-review-service/internal/service/review/domain"
)

// fakeStateStore 是 ReviewStateStore 的内存实现。
type fakeStateStore struct {
	records map[string]*domain.Record
	txnErr  error
}

func newFakeStateStore() *fakeStateStore {
	return &fakeStateStore{records: map[string]*domain.Record{}}
}

func (f *fakeStateStore) GetByReviewID(_ context.Context, id string) (*domain.Record, error) {
	r, ok := f.records[id]
	if !ok {
		return nil, repoerr.ErrNotFound
	}
	return r, nil
}

func (f *fakeStateStore) Create(_ context.Context, r *domain.Record) error {
	f.records[r.ID] = r
	return nil
}

func (f *fakeStateStore) Save(_ context.Context, r *domain.Record) error {
	f.records[r.ID] = r
	return nil
}

func (f *fakeStateStore) UpdateInTransaction(_ context.Context, id string, fn func(*domain.Record) error) error {
	if f.txnErr != nil {
		return f.txnErr
	}
	r, ok := f.records[id]
	if !ok {
		return repoerr.ErrNotFound
	}
	return fn(r)
}

func TestWriteReviewStateCreateMissing(t *testing.T) {
	store := newFakeStateStore()
	req := domain.Request{Provider: "github", Repo: "o/r"}
	resp := domain.Response{
		Status:  "success",
		Summary: "ok",
		Report:  domain.Report{Score: domain.Score{Value: 88, Grade: "A"}},
	}

	if err := WriteReviewState(context.Background(), store, "rv1", req, resp, []byte("raw")); err != nil {
		t.Fatalf("WriteReviewState: %v", err)
	}
	rec := store.records["rv1"]
	if rec == nil {
		t.Fatal("record not created")
	}
	if rec.Score != 88 || rec.Grade != "A" {
		t.Fatalf("create path did not persist Score/Grade: got %v/%q", rec.Score, rec.Grade)
	}
	if rec.Summary != "ok" || string(rec.RawOCR) != "raw" {
		t.Fatalf("create path did not persist Summary/RawOCR: %q %q", rec.Summary, rec.RawOCR)
	}
}

func TestWriteReviewStateUpdatesScoreGradeSummary(t *testing.T) {
	store := newFakeStateStore()
	store.records["rv1"] = &domain.Record{ID: "rv1", Status: "running"}
	req := domain.Request{Provider: "github", Repo: "o/r"}
	resp := domain.Response{
		Status:  "success",
		Summary: "done",
		Report:  domain.Report{Score: domain.Score{Value: 75, Grade: "B"}},
	}

	if err := WriteReviewState(context.Background(), store, "rv1", req, resp, nil); err != nil {
		t.Fatalf("WriteReviewState: %v", err)
	}
	rec := store.records["rv1"]
	// 回归锚点：旧 review.Service 版本不写 Score/Grade/Summary，两条路径语义分叉。
	if rec.Score != 75 || rec.Grade != "B" || rec.Summary != "done" {
		t.Fatalf("update path did not persist Score/Grade/Summary: got %v/%q/%q", rec.Score, rec.Grade, rec.Summary)
	}
	// 状态变迁应追加一条历史事件。
	if len(rec.Response.History) != 1 || rec.Response.History[0].Stage != "success" {
		t.Fatalf("expected one history event for status transition, got %+v", rec.Response.History)
	}
}

func TestWriteReviewStatePreservesHistory(t *testing.T) {
	store := newFakeStateStore()
	existing := domain.Record{ID: "rv1", Status: "running"}
	existing.Response.History = []domain.Event{{Stage: "running", Status: "running"}}
	store.records["rv1"] = &existing

	resp := domain.Response{Status: "failed", Summary: "review failed"}
	if err := WriteReviewState(context.Background(), store, "rv1", domain.Request{}, resp, nil); err != nil {
		t.Fatalf("WriteReviewState: %v", err)
	}
	rec := store.records["rv1"]
	if len(rec.Response.History) != 2 {
		t.Fatalf("history not preserved+appended: %+v", rec.Response.History)
	}
	if rec.Response.History[0].Stage != "running" || rec.Response.History[1].Stage != "failed" {
		t.Fatalf("unexpected history order: %+v", rec.Response.History)
	}
}

func TestFailureReporterPersistsFailureWithoutSideEffects(t *testing.T) {
	store := newFakeStateStore()
	// notifier/poster 均为 nil：只落库，不 panic。
	f := NewFailureReporter(store, nil, nil)
	f.Report(context.Background(), "rv1", domain.Request{Provider: "github", Repo: "o/r"}, "worker", errors.New("boom"), domain.OCRMetadata{})

	rec := store.records["rv1"]
	if rec == nil {
		t.Fatal("failure not persisted")
	}
	if rec.Status != "failed" {
		t.Fatalf("status = %q, want failed", rec.Status)
	}
	if !strings.Contains(rec.Response.Error, "boom") {
		t.Fatalf("error not persisted: %q", rec.Response.Error)
	}
	if rec.Response.Gate.Status != domain.GateError {
		t.Fatalf("gate status = %q, want error", rec.Response.Gate.Status)
	}
}
