package stats

import (
	"context"
	"time"

	"github.com/yi-nology/code-review-service/internal/repository"
	"github.com/yi-nology/code-review-service/internal/domain/stats"
	"github.com/yi-nology/code-review-service/internal/service/review/domain"
	"github.com/oekit/pkg/slicex"
)

type Service struct {
	repo       repository.ContributorStatsRepository
	reviewRepo repository.ReviewRepository
}

func NewService(repo repository.ContributorStatsRepository, reviewRepo ...repository.ReviewRepository) *Service {
	s := &Service{repo: repo}
	if len(reviewRepo) > 0 {
		s.reviewRepo = reviewRepo[0]
	}
	return s
}

func parseTime(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}
	}
	return t
}

func (s *Service) ListContributors(ctx context.Context, provider, repo, from, to, sort, order string, page, pageSize int) (*ContributorListResult, error) {
	result, err := s.repo.ListContributors(ctx, stats.ContributorStatsOptions{
		Provider: provider,
		Repo:     repo,
		From:     parseTime(from),
		To:       parseTime(to),
		Sort:     sort,
		Order:    order,
		Page:     page,
		PageSize: pageSize,
	})
	if err != nil {
		return nil, err
	}
	return toContributorListResult(result), nil
}

func (s *Service) GetContributor(ctx context.Context, author, provider, repo, from, to string) (*ContributorDetail, error) {
	detail, err := s.repo.GetContributor(ctx, stats.ContributorStatsOptions{
		Author:   author,
		Provider: provider,
		Repo:     repo,
		From:     parseTime(from),
		To:       parseTime(to),
	})
	if err != nil {
		return nil, err
	}
	if detail == nil {
		return nil, nil
	}
	return toContributorDetail(detail), nil
}

func (s *Service) GetContributorTrend(ctx context.Context, author, provider, repo, from, to string) ([]TrendPoint, error) {
	points, err := s.repo.GetContributorTrend(ctx, stats.ContributorStatsOptions{
		Author:   author,
		Provider: provider,
		Repo:     repo,
		From:     parseTime(from),
		To:       parseTime(to),
	})
	if err != nil {
		return nil, err
	}
	return slicex.Map(points, func(p stats.TrendPoint) TrendPoint { return TrendPoint(p) }), nil
}

func (s *Service) GetContributorCategories(ctx context.Context, author, provider, repo, from, to string) ([]CategoryStat, error) {
	cats, err := s.repo.GetContributorCategories(ctx, stats.ContributorStatsOptions{
		Author:   author,
		Provider: provider,
		Repo:     repo,
		From:     parseTime(from),
		To:       parseTime(to),
	})
	if err != nil {
		return nil, err
	}
	return slicex.Map(cats, func(c stats.CategoryStat) CategoryStat { return CategoryStat(c) }), nil
}

func (s *Service) ListProjects(ctx context.Context, provider, repo, from, to, sort, order string, page, pageSize int) (*ProjectListResult, error) {
	result, err := s.repo.ListProjects(ctx, domain.ProjectStatsOptions{
		Provider: provider,
		Repo:     repo,
		From:     parseTime(from),
		To:       parseTime(to),
		Sort:     sort,
		Order:    order,
		Page:     page,
		PageSize: pageSize,
	})
	if err != nil {
		return nil, err
	}
	return toProjectListResult(result), nil
}

func (s *Service) GetProject(ctx context.Context, provider, repo, from, to string) (*ProjectDetail, error) {
	detail, err := s.repo.GetProject(ctx, domain.ProjectStatsOptions{
		Provider: provider,
		Repo:     repo,
		From:     parseTime(from),
		To:       parseTime(to),
	})
	if err != nil {
		return nil, err
	}
	if detail == nil {
		return nil, nil
	}
	return toProjectDetail(detail), nil
}

func (s *Service) GetProjectTrend(ctx context.Context, provider, repo, from, to string) ([]ProjectTrendPoint, error) {
	points, err := s.repo.GetProjectTrend(ctx, domain.ProjectStatsOptions{
		Provider: provider,
		Repo:     repo,
		From:     parseTime(from),
		To:       parseTime(to),
	})
	if err != nil {
		return nil, err
	}
	return slicex.Map(points, func(p stats.ProjectTrendPoint) ProjectTrendPoint { return ProjectTrendPoint(p) }), nil
}

func (s *Service) GetProjectCategories(ctx context.Context, provider, repo, from, to string) ([]CategoryStat, error) {
	cats, err := s.repo.GetProjectCategories(ctx, domain.ProjectStatsOptions{
		Provider: provider,
		Repo:     repo,
		From:     parseTime(from),
		To:       parseTime(to),
	})
	if err != nil {
		return nil, err
	}
	return slicex.Map(cats, func(c stats.CategoryStat) CategoryStat { return CategoryStat(c) }), nil
}

func (s *Service) GetProjectFiles(ctx context.Context, provider, repo, from, to string) ([]FileRiskStat, error) {
	files, err := s.repo.GetProjectFiles(ctx, domain.ProjectStatsOptions{
		Provider: provider,
		Repo:     repo,
		From:     parseTime(from),
		To:       parseTime(to),
	})
	if err != nil {
		return nil, err
	}
	return slicex.Map(files, func(f stats.FileRiskStat) FileRiskStat { return FileRiskStat(f) }), nil
}

func (s *Service) GetProjectContributors(ctx context.Context, provider, repo, from, to string, limit int) ([]ContributorSummary, error) {
	contributors, err := s.repo.GetProjectContributors(ctx, domain.ProjectStatsOptions{
		Provider: provider,
		Repo:     repo,
		From:     parseTime(from),
		To:       parseTime(to),
	}, limit)
	if err != nil {
		return nil, err
	}
	return slicex.Map(contributors, toContributorSummary), nil
}

func (s *Service) GetProjectOCRMetrics(ctx context.Context, provider, repo, from, to string) (*ProjectOCRMetrics, error) {
	if s.reviewRepo == nil {
		return &ProjectOCRMetrics{}, nil
	}
	metrics, err := s.reviewRepo.ProjectOCRMetrics(ctx, domain.ProjectStatsOptions{
		Provider: provider,
		Repo:     repo,
		From:     parseTime(from),
		To:       parseTime(to),
	})
	if err != nil {
		return nil, err
	}
	if metrics == nil {
		return &ProjectOCRMetrics{}, nil
	}
	return &ProjectOCRMetrics{
		ReviewCount:        metrics.ReviewCount,
		TotalTokens:        metrics.TotalTokens,
		InputTokens:        metrics.InputTokens,
		OutputTokens:       metrics.OutputTokens,
		CacheReadTokens:    metrics.CacheReadTokens,
		CacheWriteTokens:   metrics.CacheWriteTokens,
		TotalReviewedFiles: metrics.TotalReviewedFiles,
		AvgTokens:          metrics.AvgTokens,
		AvgScore:           metrics.AvgScore,
		AvgFindingsDensity: metrics.AvgFindingsDensity,
		GradeDistribution:  metrics.GradeDistribution,
		ToolCalls:          slicex.Map(metrics.ToolCalls, func(t domain.ToolCallStat) ToolCallStat { return ToolCallStat(t) }),
	}, nil
}

func (s *Service) GetProjectOCRTrend(ctx context.Context, provider, repo, from, to string) ([]ProjectOCRTrendPoint, error) {
	if s.reviewRepo == nil {
		return nil, nil
	}
	points, err := s.reviewRepo.ProjectOCRTrend(ctx, domain.ProjectStatsOptions{
		Provider: provider,
		Repo:     repo,
		From:     parseTime(from),
		To:       parseTime(to),
	})
	if err != nil {
		return nil, err
	}
	return slicex.Map(points, func(p domain.ProjectOCRTrendPoint) ProjectOCRTrendPoint {
		return ProjectOCRTrendPoint(p)
	}), nil
}

func toContributorSummary(s stats.ContributorSummary) ContributorSummary {
	return ContributorSummary{
		Author:        s.Author,
		AuthorName:    s.AuthorName,
		AuthorAvatar:  s.AuthorAvatar,
		ReviewCount:   s.ReviewCount,
		AvgScore:      s.AvgScore,
		PassRate:      s.PassRate,
		TotalFindings: s.TotalFindings,
		CriticalCount: s.CriticalCount,
		MajorCount:    s.MajorCount,
		MinorCount:    s.MinorCount,
		InfoCount:     s.InfoCount,
		LastReviewAt:  s.LastReviewAt,
	}
}

func toContributorDetail(d *stats.ContributorDetail) *ContributorDetail {
	return &ContributorDetail{
		ContributorSummary: toContributorSummary(d.ContributorSummary),
		MedianScore:        d.MedianScore,
		MaxScore:           d.MaxScore,
		MinScore:           d.MinScore,
		GradeDistribution:  d.GradeDistribution,
		AvgFindings:        d.AvgFindings,
		AvgChangedLines:    d.AvgChangedLines,
	}
}

func toContributorListResult(r *stats.ContributorListResult) *ContributorListResult {
	return &ContributorListResult{
		Contributors: slicex.Map(r.Contributors, toContributorSummary),
		Total:        r.Total,
		Page:         r.Page,
		PageSize:     r.PageSize,
	}
}

func toProjectSummary(s stats.ProjectSummary) ProjectSummary {
	return ProjectSummary{
		Provider:      s.Provider,
		Repo:          s.Repo,
		ReviewCount:   s.ReviewCount,
		AvgScore:      s.AvgScore,
		PassRate:      s.PassRate,
		TotalFindings: s.TotalFindings,
		CriticalCount: s.CriticalCount,
		MajorCount:    s.MajorCount,
		MinorCount:    s.MinorCount,
		InfoCount:     s.InfoCount,
		LastReviewAt:  s.LastReviewAt,
	}
}

func toProjectDetail(d *stats.ProjectDetail) *ProjectDetail {
	return &ProjectDetail{
		ProjectSummary:    toProjectSummary(d.ProjectSummary),
		MedianScore:       d.MedianScore,
		MaxScore:          d.MaxScore,
		MinScore:          d.MinScore,
		GradeDistribution: d.GradeDistribution,
		AvgFindings:       d.AvgFindings,
		AvgChangedLines:   d.AvgChangedLines,
	}
}

func toProjectListResult(r *stats.ProjectListResult) *ProjectListResult {
	return &ProjectListResult{
		Projects: slicex.Map(r.Projects, toProjectSummary),
		Total:    r.Total,
		Page:     r.Page,
		PageSize: r.PageSize,
	}
}
