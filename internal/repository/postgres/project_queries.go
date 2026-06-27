package postgres

import (
	"context"

	"github.com/yi-nology/code-review-service/internal/repository/postgres/models"
	"github.com/oekit/logx"
	"github.com/oekit/pkg/mathx"
)

func (r *ContributorStatsRepo) ListProjects(ctx context.Context, opts models.ProjectStatsOptions) (*models.ProjectListResult, error) {
	if opts.Page <= 0 {
		opts.Page = 1
	}
	if opts.PageSize <= 0 || opts.PageSize > 200 {
		opts.PageSize = 50
	}

	var total int64
	err := r.db.WithContext(ctx).Raw(`
		SELECT COUNT(*) FROM (
			SELECT provider, repo FROM reviews
			WHERE `+baseReviewWhere+` `+projectFilterClause(opts)+`
			GROUP BY provider, repo
		) projects
	`, projectFilterArgs(opts)...).Scan(&total).Error
	if err != nil {
		return nil, err
	}

	orderDir := "DESC"
	if opts.Order == "asc" {
		orderDir = "ASC"
	}

	var rows []projectRow
	err = r.db.WithContext(ctx).Raw(`
		SELECT `+projectSelectCols+`
		FROM reviews
		WHERE `+baseReviewWhere+` `+projectFilterClause(opts)+`
		GROUP BY provider, repo
		ORDER BY `+r.projectSortColumn(opts.Sort)+` `+orderDir+`, provider ASC, repo ASC
		LIMIT ? OFFSET ?
	`, append(projectFilterArgs(opts), opts.PageSize, (opts.Page-1)*opts.PageSize)...).Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	return &models.ProjectListResult{
		Projects: projectRowsToSummaries(rows), Total: total, Page: opts.Page, PageSize: opts.PageSize,
	}, nil
}

func (r *ContributorStatsRepo) GetProject(ctx context.Context, opts models.ProjectStatsOptions) (*models.ProjectDetail, error) {
	var row projectRow
	err := r.db.WithContext(ctx).Raw(`
		SELECT `+projectSelectCols+`
		FROM reviews
		WHERE provider = ? AND repo = ? AND `+baseReviewWhere+`
			`+projectFilterClauseWithoutProject(opts)+`
		GROUP BY provider, repo
	`, append([]interface{}{opts.Provider, opts.Repo}, projectFilterArgsWithoutProject(opts)...)...).Scan(&row).Error
	if err != nil {
		return nil, err
	}
	if row.Provider == "" || row.Repo == "" {
		return nil, nil
	}

	var maxScore, minScore int
	var medianScore, avgFindings, avgChangedLines float64
	if err := r.db.WithContext(ctx).Raw(`
		SELECT
			COALESCE(MAX(NULLIF(response->'report'->'score'->>'value','')::int), 0),
			COALESCE(MIN(NULLIF(response->'report'->'score'->>'value','')::int), 0),
			COALESCE(PERCENTILE_CONT(0.5) WITHIN GROUP (ORDER BY NULLIF(response->'report'->'score'->>'value','')::int), 0),
			COALESCE(AVG(jsonb_array_length(response->'findings')), 0),
			COALESCE(AVG(NULLIF(response->'report'->'score'->>'changed_lines','')::int), 0)
		FROM reviews
		WHERE provider = ? AND repo = ? AND `+baseReviewWhere+`
			`+projectFilterClauseWithoutProject(opts)+`
	`, append([]interface{}{opts.Provider, opts.Repo}, projectFilterArgsWithoutProject(opts)...)...).Row().Scan(&maxScore, &minScore, &medianScore, &avgFindings, &avgChangedLines); err != nil {
		logx.Warnf("failed to scan project stats: %v", err)
	}

	gradeDist := r.getGradeDistribution(ctx,
		"provider = ? AND repo = ? AND "+baseReviewWhere+projectFilterClauseWithoutProject(opts),
		append([]interface{}{opts.Provider, opts.Repo}, projectFilterArgsWithoutProject(opts)...)...)

	return &models.ProjectDetail{
		ProjectSummary: projectRowToSummary(row), MedianScore: mathx.Round(medianScore, 2),
		MaxScore: maxScore, MinScore: minScore, GradeDistribution: gradeDist,
		AvgFindings: mathx.Round(avgFindings, 2), AvgChangedLines: mathx.Round(avgChangedLines, 2),
	}, nil
}

func (r *ContributorStatsRepo) GetProjectTrend(ctx context.Context, opts models.ProjectStatsOptions) ([]models.ProjectTrendPoint, error) {
	var rows []struct {
		Period        string  `gorm:"column:period"`
		AvgScore      float64 `gorm:"column:avg_score"`
		ReviewCount   int     `gorm:"column:review_count"`
		TotalFindings int     `gorm:"column:total_findings"`
	}
	err := r.db.WithContext(ctx).Raw(`
		SELECT TO_CHAR(date_trunc('week', created_at), 'YYYY-MM-DD') AS period,
			COALESCE(AVG(NULLIF(response->'report'->'score'->>'value','')::int), 0) AS avg_score,
			COUNT(*) AS review_count,
			COALESCE(SUM(jsonb_array_length(response->'findings')), 0) AS total_findings
		FROM reviews
		WHERE provider = ? AND repo = ? AND `+baseReviewWhere+`
			`+projectFilterClauseWithoutProject(opts)+`
		GROUP BY period ORDER BY period ASC
	`, append([]interface{}{opts.Provider, opts.Repo}, projectFilterArgsWithoutProject(opts)...)...).Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	points := make([]models.ProjectTrendPoint, 0, len(rows))
	for _, row := range rows {
		points = append(points, models.ProjectTrendPoint{
			Period: row.Period, AvgScore: mathx.Round(row.AvgScore, 2),
			ReviewCount: row.ReviewCount, TotalFindings: row.TotalFindings,
		})
	}
	return points, nil
}

func (r *ContributorStatsRepo) GetProjectCategories(ctx context.Context, opts models.ProjectStatsOptions) ([]models.CategoryStat, error) {
	return r.getCategories(ctx,
		"provider = ? AND repo = ? AND "+baseReviewWhere+projectFilterClauseWithoutProject(opts),
		append([]interface{}{opts.Provider, opts.Repo}, projectFilterArgsWithoutProject(opts)...)...)
}

func (r *ContributorStatsRepo) GetProjectFiles(ctx context.Context, opts models.ProjectStatsOptions) ([]models.FileRiskStat, error) {
	var rows []struct {
		Path          string `gorm:"column:path"`
		Count         int    `gorm:"column:cnt"`
		CriticalCount int    `gorm:"column:critical_count"`
		MajorCount    int    `gorm:"column:major_count"`
	}
	err := r.db.WithContext(ctx).Raw(`
		SELECT
			finding->>'path' AS path, COUNT(*) AS cnt,
			COALESCE(SUM(CASE WHEN finding->>'severity' = 'critical' THEN 1 ELSE 0 END), 0) AS critical_count,
			COALESCE(SUM(CASE WHEN finding->>'severity' = 'major' THEN 1 ELSE 0 END), 0) AS major_count
		FROM reviews, jsonb_array_elements(response->'findings') AS finding
		WHERE provider = ? AND repo = ? AND `+baseReviewWhere+`
			AND COALESCE(finding->>'path', '') != ''
			`+projectFilterClauseWithoutProject(opts)+`
		GROUP BY path ORDER BY critical_count DESC, major_count DESC, cnt DESC
		LIMIT 20
	`, append([]interface{}{opts.Provider, opts.Repo}, projectFilterArgsWithoutProject(opts)...)...).Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	stats := make([]models.FileRiskStat, 0, len(rows))
	for _, row := range rows {
		stats = append(stats, models.FileRiskStat{
			Path: row.Path, Count: row.Count, CriticalCount: row.CriticalCount, MajorCount: row.MajorCount,
		})
	}
	return stats, nil
}

func (r *ContributorStatsRepo) GetProjectContributors(ctx context.Context, opts models.ProjectStatsOptions, limit int) ([]models.ContributorSummary, error) {
	if limit <= 0 || limit > 100 {
		limit = 10
	}
	var rows []contributorRow
	err := r.db.WithContext(ctx).Raw(`
		SELECT `+contributorSelectCols+`
		FROM reviews
		WHERE provider = ? AND repo = ? AND author != '' AND `+baseReviewWhere+`
			`+projectFilterClauseWithoutProject(opts)+`
		GROUP BY author ORDER BY review_count DESC, avg_score DESC, author ASC
		LIMIT ?
	`, append(append([]interface{}{opts.Provider, opts.Repo}, projectFilterArgsWithoutProject(opts)...), limit)...).Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	return rowsToSummaries(rows), nil
}
