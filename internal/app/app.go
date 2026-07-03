package app

import (
	"context"

	"github.com/yi-nology/code-review-service/internal/domain/platform"
	"github.com/yi-nology/code-review-service/internal/service/review"
	"github.com/yi-nology/code-review-service/internal/service/review/domain"

	"github.com/yi-nology/git-platform-sdk/provider"
)

type ReviewService interface {
	Create(context.Context, domain.Request, string) (domain.Response, error)
	Get(context.Context, string) (domain.Record, bool, error)
	List(context.Context, review.ListOptions) (review.ListResult, error)
	Preview(context.Context, domain.Request) ([]string, error)
	Scan(context.Context, domain.ScanRequest) (*domain.ScanResult, error)
	CompareModels(context.Context, string, review.ModelCompareOptions) (*review.ModelCompareResult, bool, error)
	Requeue(context.Context, string) (domain.Response, error)
	Cancel(context.Context, string) (domain.Response, error)
}

type GitProviderManager interface {
	AddOrUpdateFromDB(p *platform.Platform) error
	Remove(name string)
	TestWithToken(ctx context.Context, platform, baseURL, token string, skipTLS bool) (*platform.TestResult, error)
	HasProvider(platform string) bool
	ListRepos(ctx context.Context, platform string, opts provider.ListRepoOptions) ([]*provider.PlatformRepo, error)
	ListBranches(ctx context.Context, platform, owner, repo string) ([]*provider.PlatformBranch, error)
	ListCRs(ctx context.Context, platform string, opts provider.ListCROptions) ([]*provider.ChangeRequest, int, error)
	CreateCR(ctx context.Context, platform string, opts provider.CreateCROptions) (*provider.ChangeRequest, error)
	GetCR(ctx context.Context, platform, owner, repo string, number int) (*provider.ChangeRequest, error)
	GetBranchSHA(ctx context.Context, platform, owner, repo, branch string) (string, error)
}
