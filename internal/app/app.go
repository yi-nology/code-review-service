package app

import (
	"context"

	"github.com/yi-nology/code-review-service/internal/domain/platform"
	"github.com/yi-nology/code-review-service/internal/service/review/domain"

	"github.com/yi-nology/git-platform-sdk/provider"
)

// ReviewService 是 review 业务服务的抽象契约，供 handler / mcp 等消费方依赖。
// 仅依赖 domain（纯类型包），不依赖 internal/service/review 实现层，
// 避免抽象层反向依赖实现层。
type ReviewService interface {
	Create(context.Context, domain.Request, string) (domain.Response, error)
	Get(context.Context, string) (domain.Record, bool, error)
	List(context.Context, domain.ListRequest) (domain.ListResponse, error)
	Preview(context.Context, domain.Request) ([]string, error)
	Scan(context.Context, domain.ScanRequest) (*domain.ScanResult, error)
	// DelegatePreview / DelegateRule 暴露 OCR v1.7.11+ delegate 子命令（host-agent 范式，不调 LLM）。
	DelegatePreview(context.Context, domain.Request, []string) (*domain.DelegatePreviewResult, error)
	DelegateRule(context.Context, domain.Request, []string) (*domain.DelegateRuleResult, error)
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
	GetCR(ctx context.Context, platform, owner, repo, number string) (*provider.ChangeRequest, error)
	GetBranchSHA(ctx context.Context, platform, owner, repo, branch string) (string, error)
}
