package app

import (
	"context"

	"github.com/yi-nology/code-review-service/internal/domain/platform"
	"github.com/yi-nology/code-review-service/internal/service/review/domain"

	"github.com/yi-nology/go-git-platform/provider"
)

// 本包是 handler / mcp 等消费方依赖的 service 抽象契约。
// 接口按消费方角色拆窄（Go 惯例：依赖自己用到的那部分语义），组合接口保留给
// 需要全量的装配点（handler / mcp RegisterReviewTools）。仅依赖 domain /
// SDK 抽象类型，不依赖 internal/service/* 实现层，避免抽象层反向依赖实现层。

// ---- review 服务角色 ----

// ReviewCreator 创建评审（API 直连 / mcp create_review）。
type ReviewCreator interface {
	Create(context.Context, domain.Request, string) (domain.Response, error)
}

// ReviewReader 查询评审与预览/扫描/OCR delegate。
type ReviewReader interface {
	Get(context.Context, string) (domain.Record, bool, error)
	List(context.Context, domain.ListRequest) (domain.ListResponse, error)
	Preview(context.Context, domain.Request) ([]string, error)
	Scan(context.Context, domain.ScanRequest) (*domain.ScanResult, error)
	// DelegatePreview / DelegateRule 暴露 OCR v1.7.11+ delegate 子命令（host-agent 范式，不调 LLM）。
	DelegatePreview(context.Context, domain.Request, []string) (*domain.DelegatePreviewResult, error)
	DelegateRule(context.Context, domain.Request, []string) (*domain.DelegateRuleResult, error)
}

// ReviewLifecycle 评审生命周期操作（取消/重跑）。
type ReviewLifecycle interface {
	Requeue(context.Context, string) (domain.Response, error)
	Cancel(context.Context, string) (domain.Response, error)
}

// ReviewService 全量契约（三个角色的组合）。新消费方优先依赖窄角色接口。
type ReviewService interface {
	ReviewCreator
	ReviewReader
	ReviewLifecycle
}

// ---- git 平台管理角色 ----

// PlatformReader 平台只读查询（仓库/分支/PR/MR 浏览、SHA 解析）。
type PlatformReader interface {
	HasProvider(platform string) bool
	ListRepos(ctx context.Context, platform string, opts provider.ListRepoOptions) ([]*provider.PlatformRepo, error)
	ListBranches(ctx context.Context, platform, owner, repo string) ([]*provider.PlatformBranch, error)
	ListCRs(ctx context.Context, platform string, opts provider.ListCROptions) ([]*provider.ChangeRequest, int, error)
	GetCR(ctx context.Context, platform, owner, repo, number string) (*provider.ChangeRequest, error)
	GetBranchSHA(ctx context.Context, platform, owner, repo, branch string) (string, error)
}

// PlatformCRWriter 创建 PR/MR（suggestion patch 应用路径）。
type PlatformCRWriter interface {
	CreateCR(ctx context.Context, platform string, opts provider.CreateCROptions) (*provider.ChangeRequest, error)
}

// PlatformAdmin 平台安装管理（web console 平台设置页）。
type PlatformAdmin interface {
	AddOrUpdateFromDB(p *platform.Platform) error
	Remove(name string)
	TestWithToken(ctx context.Context, platform, baseURL, token string, skipTLS bool) (*platform.TestResult, error)
}

// GitProviderManager 全量契约（三个角色的组合）。
type GitProviderManager interface {
	PlatformReader
	PlatformCRWriter
	PlatformAdmin
}
