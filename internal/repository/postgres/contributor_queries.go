package postgres

import (
	"context"
	"fmt"

	"github.com/oekit/logx"
	"github.com/oekit/pkg/mathx"
	"github.com/yi-nology/code-review-service/internal/domain/stats"
	"github.com/yi-nology/code-review-service/internal/repository/postgres/models"

	gormLib "gorm.io/gorm"
)

func (r *ContributorStatsRepo) ListContributors(ctx context.Context, opts stats.ContributorStatsOptions) (*stats.ContributorListResult, error) {
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

	return &stats.ContributorListResult{
		Contributors: rowsToSummaries(rows), Total: total, Page: opts.Page, PageSize: opts.PageSize,
	}, nil
}

func (r *ContributorStatsRepo) GetContributor(ctx context.Context, opts stats.ContributorStatsOptions) (*stats.ContributorDetail, error) {
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
			COALESCE(MAX(CASE WHEN response->'report'->'score'->>'value' ~ '^[0-9]+$' THEN (response->'report'->'score'->>'value')::int END), 0),
			COALESCE(MIN(CASE WHEN response->'report'->'score'->>'value' ~ '^[0-9]+$' THEN (response->'report'->'score'->>'value')::int END), 0),
			COALESCE(PERCENTILE_CONT(0.5) WITHIN GROUP (ORDER BY CASE WHEN response->'report'->'score'->>'value' ~ '^[0-9]+$' THEN (response->'report'->'score'->>'value')::int END), 0),
			COALESCE(AVG(jsonb_array_length(response->'findings')), 0),
			COALESCE(AVG(CASE WHEN response->'report'->'score'->>'changed_lines' ~ '^[0-9]+$' THEN (response->'report'->'score'->>'changed_lines')::int END), 0)
		FROM reviews WHERE author = ? AND `+baseReviewWhere+`
	`, opts.Author).Row().Scan(&maxScore, &minScore, &medianScore, &avgFindings, &avgChangedLines); err != nil {
		return nil, fmt.Errorf("query contributor detail stats: %w", err)
	}

	gradeDist := r.getGradeDistribution(ctx, "author = ?", opts.Author)

	return &stats.ContributorDetail{
		ContributorSummary: rowToSummary(row), MedianScore: mathx.Round(medianScore, 2),
		MaxScore: maxScore, MinScore: minScore, GradeDistribution: gradeDist,
		AvgFindings: mathx.Round(avgFindings, 2), AvgChangedLines: mathx.Round(avgChangedLines, 2),
	}, nil
}

func (r *ContributorStatsRepo) GetContributorTrend(ctx context.Context, opts stats.ContributorStatsOptions) ([]stats.TrendPoint, error) {
	var rows []struct {
		Period      string  `gorm:"column:period"`
		AvgScore    float64 `gorm:"column:avg_score"`
		ReviewCount int     `gorm:"column:review_count"`
	}
	err := r.db.WithContext(ctx).Raw(`
		SELECT TO_CHAR(date_trunc('week', created_at), 'YYYY-MM-DD') AS period,
			COALESCE(AVG(CASE WHEN response->'report'->'score'->>'value' ~ '^[0-9]+$' THEN (response->'report'->'score'->>'value')::int END), 0) AS avg_score,
			COUNT(*) AS review_count
		FROM reviews
		WHERE author = ? AND `+baseReviewWhere+` `+filterClause(opts)+`
		GROUP BY period ORDER BY period ASC
	`, append([]interface{}{opts.Author}, filterArgs(opts)...)...).Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	points := make([]stats.TrendPoint, 0, len(rows))
	for _, row := range rows {
		points = append(points, stats.TrendPoint{
			Period: row.Period, AvgScore: mathx.Round(row.AvgScore, 2), ReviewCount: row.ReviewCount,
		})
	}
	return points, nil
}

func (r *ContributorStatsRepo) GetContributorCategories(ctx context.Context, opts stats.ContributorStatsOptions) ([]stats.CategoryStat, error) {
	return r.getCategories(ctx, "author = ? AND "+baseReviewWhere+filterClause(opts),
		append([]interface{}{opts.Author}, filterArgs(opts)...))
}

func (r *ContributorStatsRepo) getGradeDistribution(ctx context.Context, where string, args ...interface{}) map[string]int {
	type gradeRow struct {
		Grade string `gorm:"column:grade"`
		Count int    `gorm:"column:cnt"`
	}
	var rows []gradeRow
	if err := r.db.WithContext(ctx).Raw(`
		SELECT response->'report'->'score'->>'grade' AS grade, COUNT(*) AS cnt
		FROM reviews WHERE `+where+` GROUP BY 1
	`, args...).Scan(&rows).Error; err != nil {
		logx.Warnf("failed to query grade distribution: %v", err)
		return map[string]int{}
	}

	dist := make(map[string]int)
	for _, gr := range rows {
		if gr.Grade != "" {
			dist[gr.Grade] = gr.Count
		}
	}
	return dist
}

func (r *ContributorStatsRepo) getCategories(ctx context.Context, where string, args ...interface{}) ([]stats.CategoryStat, error) {
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

	items := make([]stats.CategoryStat, 0, len(rows))
	for _, row := range rows {
		cat := row.Category
		if cat == "" {
			cat = "可维护性"
		}
		items = append(items, stats.CategoryStat{Category: cat, Count: row.Count})
	}
	return items, nil
}
