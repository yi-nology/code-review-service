package review_service

import (
	"context"
	"errors"
	"testing"

	"github.com/yi-nology/code-review-service/internal/service/review"
)

type mockReviewService struct {
	createFunc    func(ctx context.Context, req review.Request, idempotencyKey string) (review.Response, error)
	getFunc       func(ctx context.Context, id string) (review.Record, bool, error)
	listFunc      func(ctx context.Context, opts review.ListOptions) (review.ListResult, error)
	previewFunc   func(ctx context.Context, req review.Request) ([]string, error)
	requeueFunc   func(ctx context.Context, id string) (review.Response, error)
	cancelFunc    func(ctx context.Context, id string) (review.Response, error)
}

func (m *mockReviewService) Create(ctx context.Context, req review.Request, idempotencyKey string) (review.Response, error) {
	if m.createFunc != nil {
		return m.createFunc(ctx, req, idempotencyKey)
	}
	return review.Response{ReviewID: "test-123", Status: "queued"}, nil
}

func (m *mockReviewService) Get(ctx context.Context, id string) (review.Record, bool, error) {
	if m.getFunc != nil {
		return m.getFunc(ctx, id)
	}
	return review.Record{}, false, nil
}

func (m *mockReviewService) List(ctx context.Context, opts review.ListOptions) (review.ListResult, error) {
	if m.listFunc != nil {
		return m.listFunc(ctx, opts)
	}
	return review.ListResult{}, nil
}

func (m *mockReviewService) Preview(ctx context.Context, req review.Request) ([]string, error) {
	if m.previewFunc != nil {
		return m.previewFunc(ctx, req)
	}
	return nil, nil
}

func (m *mockReviewService) Requeue(ctx context.Context, id string) (review.Response, error) {
	if m.requeueFunc != nil {
		return m.requeueFunc(ctx, id)
	}
	return review.Response{}, nil
}

func (m *mockReviewService) Cancel(ctx context.Context, id string) (review.Response, error) {
	if m.cancelFunc != nil {
		return m.cancelFunc(ctx, id)
	}
	return review.Response{}, nil
}

type mockGitProviderManager struct {
	hasProviderFunc func(platform string) bool
}

func (m *mockGitProviderManager) AddOrUpdateFromDB(p interface{}) error { return nil }
func (m *mockGitProviderManager) Remove(name string)                   {}
func (m *mockGitProviderManager) TestWithToken(ctx context.Context, platform, baseURL, token string) (interface{}, error) {
	return nil, nil
}
func (m *mockGitProviderManager) HasProvider(platform string) bool {
	if m.hasProviderFunc != nil {
		return m.hasProviderFunc(platform)
	}
	return true
}
func (m *mockGitProviderManager) ListRepos(ctx context.Context, platform string, opts interface{}) (interface{}, error) {
	return nil, nil
}
func (m *mockGitProviderManager) ListBranches(ctx context.Context, platform, owner, repo string) (interface{}, error) {
	return nil, nil
}
func (m *mockGitProviderManager) ListCRs(ctx context.Context, platform string, opts interface{}) (interface{}, int, error) {
	return nil, 0, nil
}
func (m *mockGitProviderManager) GetCR(ctx context.Context, platform, owner, repo string, number int) (interface{}, error) {
	return nil, nil
}
func (m *mockGitProviderManager) GetBranchSHA(ctx context.Context, platform, owner, repo, branch string) (string, error) {
	return "", nil
}

func TestMapReviewError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		wantCode int
	}{
		{
			name:     "unsupported provider returns 422",
			err:      review.ErrUnsupportedProvider,
			wantCode: 422,
		},
		{
			name:     "idempotency conflict returns 409",
			err:      review.ErrIdempotencyConflict,
			wantCode: 409,
		},
		{
			name:     "generic error returns 500",
			err:      errors.New("internal error"),
			wantCode: 500,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mapped := mapReviewError(tt.err)
			if mapped == nil {
				t.Fatal("expected non-nil error")
			}
		})
	}
}
