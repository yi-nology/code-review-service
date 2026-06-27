package postgres

import (
	"context"

	"github.com/yi-nology/code-review-service/internal/repository/postgres/models"
	"github.com/oekit/logx"
	"github.com/oekit/pkg/mathx"

	gormLib "gorm.io/gorm"
)

func (r *ContributorStatsRepo) ListContributors(ctx context.Context, opts models.ContributorStatsOptions) (*models.ContributorListResult, error) {
	if opts.Page <= 0 {
		opts.Page = 1
	}
	if opts.PageSize <= 0 || opts.PageSize > 200 {
		opts.PageSize = 50
	}

	base := r.applyFilters(r.db.WithContext(ctx).Model(&models.Review{}), opts)

	var total int64
	if err := base.Session(&gormLib.Session{}).Distinct("author").Count(&total).Error; err != nil {
		return nil, err
	}

	orderDir := "DESC"
	if opts.Order == "asc" {
		orderDir = "ASC"
	}

	var rows []contributorRow
	err := r.db.WithContext(ctx).Raw(`
		SELECT `+contributorSelectCols+`
		FROM reviews
		WHERE author != '' AND `+baseReviewWhere+`
			`+filterClause(opts)+`
		GROUP BY author
		ORDER BY `+r.sortColumn(opts.Sort)+` `+orderDir+`, author ASC
		LIMIT ? OFFSET ?
	`, append(filterArgs(opts), opts.PageSize, (opts.Page-1)*opts.PageSize)...).Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	return &models.ContributorListResult{
		Contributors: rowsToSummaries(rows), Total: total, Page: opts.Page, PageSize: opts.PageSize,
	}, nil
}

func (r *ContributorStatsRepo) GetContributor(ctx context.Context, opts models.ContributorStatsOptions) (*models.ContributorDetail, error) {
	var row contributorRow
	err := r.db.WithContext(ctx).Raw(`
		SELECT `+contributorSelectCols+`
		FROM reviews
		WHERE author = ? AND `+baseReviewWhere+` `+filterClause(opts)+`
		GROUP BY author
	`, append([]interface{}{opts.Author}, filterArgs(opts)...)...).Scan(&row).Error
	if err != nil {
		return nil, err
	}
	if row.Author == "" {
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
		FROM reviews WHERE author = ? AND `+baseReviewWhere+`
	`, opts.Author).Row().Scan(&maxScore, &minScore, &medianScore, &avgFindings, &avgChangedLines); err != nil {
		logx.Warnf("failed to scan contributor stats: %v", err)
	}

	gradeDist := r.getGradeDistribution(ctx, "author = ?", opts.Author)

	return &models.ContributorDetail{
		ContributorSummary: rowToSummary(row), MedianScore: mathx.Round(medianScore, 2),
		MaxScore: maxScore, MinScore: minScore, GradeDistribution: gradeDist,
		AvgFindings: mathx.Round(avgFindings, 2), AvgChangedLines: mathx.Round(avgChangedLines, 2),
	}, nil
}

func (r *ContributorStatsRepo) GetContributorTrend(ctx context.Context, opts models.ContributorStatsOptions) ([]models.TrendPoint, error) {
	var rows []struct {
		Period      string  `gorm:"column:period"`
		AvgScore    float64 `gorm:"column:avg_score"`
		ReviewCount int     `gorm:"column:review_count"`
	}
	err := r.db.WithContext(ctx).Raw(`
		SELECT TO_CHAR(date_trunc('week', created_at), 'YYYY-MM-DD') AS period,
			COALESCE(AVG(NULLIF(response->'report'->'score'->>'value','')::int), 0) AS avg_score,
			COUNT(*) AS review_count
		FROM reviews
		WHERE author = ? AND `+baseReviewWhere+` `+filterClause(opts)+`
		GROUP BY period ORDER BY period ASC
	`, append([]interface{}{opts.Author}, filterArgs(opts)...)...).Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	points := make([]models.TrendPoint, 0, len(rows))
	for _, row := range rows {
		points = append(points, models.TrendPoint{
			Period: row.Period, AvgScore: mathx.Round(row.AvgScore, 2), ReviewCount: row.ReviewCount,
		})
	}
	return points, nil
}

func (r *ContributorStatsRepo) GetContributorCategories(ctx context.Context, opts models.ContributorStatsOptions) ([]models.CategoryStat, error) {
	return r.getCategories(ctx, "author = ? AND "+baseReviewWhere+filterClause(opts),
		append([]interface{}{opts.Author}, filterArgs(opts)...))
}

func (r *ContributorStatsRepo) getGradeDistribution(ctx context.Context, where string, args ...interface{}) map[string]int {
	type gradeRow struct {
		Grade string `gorm:"column:grade"`
		Count int    `gorm:"column:cnt"`
	}
	var rows []gradeRow
	r.db.WithContext(ctx).Raw(`
		SELECT response->'report'->'score'->>'grade' AS grade, COUNT(*) AS cnt
		FROM reviews WHERE `+where+` GROUP BY grade
	`, args...).Scan(&rows)

	dist := make(map[string]int)
	for _, gr := range rows {
		dist[gr.Grade] = gr.Count
	}
	return dist
}

func (r *ContributorStatsRepo) getCategories(ctx context.Context, where string, args ...interface{}) ([]models.CategoryStat, error) {
	var rows []struct {
		Category string `gorm:"column:category"`
		Count    int    `gorm:"column:cnt"`
	}
	err := r.db.WithContext(ctx).Raw(`
		SELECT finding->>'category' AS category, COUNT(*) AS cnt
		FROM reviews, jsonb_array_elements(response->'findings') AS finding
		WHERE `+where+`
		GROUP BY category ORDER BY cnt DESC
	`, args...).Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	stats := make([]models.CategoryStat, 0, len(rows))
	for _, row := range rows {
		cat := row.Category
		if cat == "" {
			cat = "可维护性"
		}
		stats = append(stats, models.CategoryStat{Category: cat, Count: row.Count})
	}
	return stats, nil
}
