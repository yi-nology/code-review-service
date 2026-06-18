package repository

import (
	"context"

	"github.com/yi-nology/code-review-service/internal/repository/postgres/models"
)

type ContributorStatsRepository interface {
	ListContributors(ctx context.Context, opts models.ContributorStatsOptions) (*models.ContributorListResult, error)
	GetContributor(ctx context.Context, opts models.ContributorStatsOptions) (*models.ContributorDetail, error)
	GetContributorTrend(ctx context.Context, opts models.ContributorStatsOptions) ([]models.TrendPoint, error)
	GetContributorCategories(ctx context.Context, opts models.ContributorStatsOptions) ([]models.CategoryStat, error)
	ListProjects(ctx context.Context, opts models.ProjectStatsOptions) (*models.ProjectListResult, error)
	GetProject(ctx context.Context, opts models.ProjectStatsOptions) (*models.ProjectDetail, error)
	GetProjectTrend(ctx context.Context, opts models.ProjectStatsOptions) ([]models.ProjectTrendPoint, error)
	GetProjectCategories(ctx context.Context, opts models.ProjectStatsOptions) ([]models.CategoryStat, error)
	GetProjectFiles(ctx context.Context, opts models.ProjectStatsOptions) ([]models.FileRiskStat, error)
	GetProjectContributors(ctx context.Context, opts models.ProjectStatsOptions, limit int) ([]models.ContributorSummary, error)
}
