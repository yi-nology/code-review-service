package app

import (
	"context"

	"github.com/yi-nology/code-review-service/internal/repository"
	"github.com/yi-nology/code-review-service/internal/repository/postgres/models"
	"github.com/yi-nology/code-review-service/internal/service/review"

	gitProvider "github.com/yi-nology/code-review-service/internal/service/provider"
	"github.com/yi-nology/git-platform-sdk/provider"

	gormLib "gorm.io/gorm"
)

type ReviewService interface {
	Create(context.Context, review.Request, string) (review.Response, error)
	Get(context.Context, string) (review.Record, bool, error)
	List(context.Context, review.ListOptions) (review.ListResult, error)
	Preview(context.Context, review.Request) ([]string, error)
	Requeue(context.Context, string) (review.Response, error)
	Cancel(context.Context, string) (review.Response, error)
}

type GitProviderManager interface {
	AddOrUpdateFromDB(p *models.GitPlatform) error
	Remove(name string)
	TestWithToken(ctx context.Context, platform, baseURL, token string) (*models.GitTestResult, error)
	HasProvider(platform string) bool
	ListRepos(ctx context.Context, platform string, opts provider.ListRepoOptions) ([]*provider.PlatformRepo, error)
	ListBranches(ctx context.Context, platform, owner, repo string) ([]*provider.PlatformBranch, error)
	ListCRs(ctx context.Context, platform string, opts provider.ListCROptions) ([]*provider.ChangeRequest, int, error)
	GetBranchSHA(ctx context.Context, platform, owner, repo, branch string) (string, error)
}

type App struct {
	DB         *gormLib.DB
	Repos      *repository.RepoContainer
	Review     ReviewService
	Provider   *gitProvider.GitProviderService
}
