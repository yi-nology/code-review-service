package repository

import (
	"context"

	"github.com/yi-nology/code-review-service/internal/domain/stats"
	"github.com/yi-nology/code-review-service/internal/service/review/domain"
)

// ContributorStatsRepository 已迁移为 domain 类型（#6 收尾）：不再依赖 postgres/models。
// ProjectStatsOptions 继续使用 domain.ProjectStatsOptions（stats.ProjectStatsOptions 的类型别名）。
type ContributorStatsRepository interface {
	ListContributors(ctx context.Context, opts stats.ContributorStatsOptions) (*stats.ContributorListResult, error)
	GetContributor(ctx context.Context, opts stats.ContributorStatsOptions) (*stats.ContributorDetail, error)
	GetContributorTrend(ctx context.Context, opts stats.ContributorStatsOptions) ([]stats.TrendPoint, error)
	GetContributorCategories(ctx context.Context, opts stats.ContributorStatsOptions) ([]stats.CategoryStat, error)
	ListProjects(ctx context.Context, opts domain.ProjectStatsOptions) (*stats.ProjectListResult, error)
	GetProject(ctx context.Context, opts domain.ProjectStatsOptions) (*stats.ProjectDetail, error)
	GetProjectTrend(ctx context.Context, opts domain.ProjectStatsOptions) ([]stats.ProjectTrendPoint, error)
	GetProjectCategories(ctx context.Context, opts domain.ProjectStatsOptions) ([]stats.CategoryStat, error)
	GetProjectFiles(ctx context.Context, opts domain.ProjectStatsOptions) ([]stats.FileRiskStat, error)
	GetProjectFindings(ctx context.Context, opts domain.ProjectStatsOptions) ([]stats.FileFinding, error)
	GetProjectContributors(ctx context.Context, opts domain.ProjectStatsOptions, limit int) ([]stats.ContributorSummary, error)
}
