package stats

import (
	"context"
	"time"

	"review-service/internal/repository"
	"review-service/internal/repository/postgres/models"
)

type Service struct {
	repo repository.ContributorStatsRepository
}

func NewService(repo repository.ContributorStatsRepository) *Service {
	return &Service{repo: repo}
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
	result, err := s.repo.ListContributors(ctx, models.ContributorStatsOptions{
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
	detail, err := s.repo.GetContributor(ctx, models.ContributorStatsOptions{
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
	points, err := s.repo.GetContributorTrend(ctx, models.ContributorStatsOptions{
		Author:   author,
		Provider: provider,
		Repo:     repo,
		From:     parseTime(from),
		To:       parseTime(to),
	})
	if err != nil {
		return nil, err
	}
	out := make([]TrendPoint, len(points))
	for i, p := range points {
		out[i] = TrendPoint(p)
	}
	return out, nil
}

func (s *Service) GetContributorCategories(ctx context.Context, author, provider, repo, from, to string) ([]CategoryStat, error) {
	cats, err := s.repo.GetContributorCategories(ctx, models.ContributorStatsOptions{
		Author:   author,
		Provider: provider,
		Repo:     repo,
		From:     parseTime(from),
		To:       parseTime(to),
	})
	if err != nil {
		return nil, err
	}
	out := make([]CategoryStat, len(cats))
	for i, c := range cats {
		out[i] = CategoryStat(c)
	}
	return out, nil
}

func toContributorSummary(s models.ContributorSummary) ContributorSummary {
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

func toContributorDetail(d *models.ContributorDetail) *ContributorDetail {
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

func toContributorListResult(r *models.ContributorListResult) *ContributorListResult {
	contributors := make([]ContributorSummary, len(r.Contributors))
	for i, c := range r.Contributors {
		contributors[i] = toContributorSummary(c)
	}
	return &ContributorListResult{
		Contributors: contributors,
		Total:        r.Total,
		Page:         r.Page,
		PageSize:     r.PageSize,
	}
}
