package postgres

import (
	"context"
	"time"

	"github.com/yi-nology/code-review-service/internal/repository/postgres/models"
	"github.com/oekit/pkg/mathx"
	"github.com/oekit/pkg/slicex"

	gormLib "gorm.io/gorm"
)

const contributorSelectCols = `
			author,
			MAX(author_name)   AS author_name,
			MAX(author_avatar) AS author_avatar,
			COUNT(*)           AS review_count,
			COALESCE(AVG(NULLIF(response->'report'->'score'->>'value','')::int), 0) AS avg_score,
			COALESCE(SUM(CASE WHEN response->'gate'->>'status' = 'passed' THEN 1 ELSE 0 END)::float / NULLIF(COUNT(*), 0), 0) AS pass_rate,
			COALESCE(SUM(jsonb_array_length(response->'findings')), 0) AS total_findings,
			COALESCE(SUM((response->'report'->'counts'->>'critical')::int), 0) AS critical_count,
			COALESCE(SUM((response->'report'->'counts'->>'major')::int), 0) AS major_count,
			COALESCE(SUM((response->'report'->'counts'->>'minor')::int), 0) AS minor_count,
			COALESCE(SUM((response->'report'->'counts'->>'info')::int), 0) AS info_count,
			MAX(created_at)    AS last_review_at`

const projectSelectCols = `
			provider,
			repo,
			COUNT(*)           AS review_count,
			COALESCE(AVG(NULLIF(response->'report'->'score'->>'value','')::int), 0) AS avg_score,
			COALESCE(SUM(CASE WHEN response->'gate'->>'status' = 'passed' THEN 1 ELSE 0 END)::float / NULLIF(COUNT(*), 0), 0) AS pass_rate,
			COALESCE(SUM(jsonb_array_length(response->'findings')), 0) AS total_findings,
			COALESCE(SUM((response->'report'->'counts'->>'critical')::int), 0) AS critical_count,
			COALESCE(SUM((response->'report'->'counts'->>'major')::int), 0) AS major_count,
			COALESCE(SUM((response->'report'->'counts'->>'minor')::int), 0) AS minor_count,
			COALESCE(SUM((response->'report'->'counts'->>'info')::int), 0) AS info_count,
			MAX(created_at)    AS last_review_at`

const baseReviewWhere = "status = 'success' AND deleted_at IS NULL"

type ContributorStatsRepo struct {
	db *gormLib.DB
}

func NewContributorStatsRepo(db *gormLib.DB) *ContributorStatsRepo {
	return &ContributorStatsRepo{db: db}
}

func (r *ContributorStatsRepo) applyFilters(db *gormLib.DB, opts models.ContributorStatsOptions) *gormLib.DB {
	db = db.Where("author != '' AND " + baseReviewWhere)
	if opts.Provider != "" {
		db = db.Where("provider = ?", opts.Provider)
	}
	if opts.Repo != "" {
		db = db.Where("repo ILIKE ?", "%"+opts.Repo+"%")
	}
	if !opts.From.IsZero() {
		db = db.Where("created_at >= ?", opts.From)
	}
	if !opts.To.IsZero() {
		db = db.Where("created_at <= ?", opts.To)
	}
	return db
}

func (r *ContributorStatsRepo) sortColumn(sort string) string {
	switch sort {
	case "review_count":
		return "review_count"
	case "pass_rate":
		return "pass_rate"
	case "total_findings":
		return "total_findings"
	default:
		return "avg_score"
	}
}

func (r *ContributorStatsRepo) projectSortColumn(sort string) string {
	switch sort {
	case "review_count":
		return "review_count"
	case "pass_rate":
		return "pass_rate"
	case "total_findings":
		return "total_findings"
	case "critical_count":
		return "critical_count"
	case "last_review_at":
		return "last_review_at"
	default:
		return "avg_score"
	}
}

type contributorRow struct {
	Author        string    `gorm:"column:author"`
	AuthorName    string    `gorm:"column:author_name"`
	AuthorAvatar  string    `gorm:"column:author_avatar"`
	ReviewCount   int       `gorm:"column:review_count"`
	AvgScore      float64   `gorm:"column:avg_score"`
	PassRate      float64   `gorm:"column:pass_rate"`
	TotalFindings int       `gorm:"column:total_findings"`
	CriticalCount int       `gorm:"column:critical_count"`
	MajorCount    int       `gorm:"column:major_count"`
	MinorCount    int       `gorm:"column:minor_count"`
	InfoCount     int       `gorm:"column:info_count"`
	LastReviewAt  time.Time `gorm:"column:last_review_at"`
}

type projectRow struct {
	Provider      string    `gorm:"column:provider"`
	Repo          string    `gorm:"column:repo"`
	ReviewCount   int       `gorm:"column:review_count"`
	AvgScore      float64   `gorm:"column:avg_score"`
	PassRate      float64   `gorm:"column:pass_rate"`
	TotalFindings int       `gorm:"column:total_findings"`
	CriticalCount int       `gorm:"column:critical_count"`
	MajorCount    int       `gorm:"column:major_count"`
	MinorCount    int       `gorm:"column:minor_count"`
	InfoCount     int       `gorm:"column:info_count"`
	LastReviewAt  time.Time `gorm:"column:last_review_at"`
}

func (r *ContributorStatsRepo) ListContributors(ctx context.Context, opts models.ContributorStatsOptions) (*models.ContributorListResult, error) {
	if opts.Page <= 0 {
		opts.Page = 1
	}
	if opts.PageSize <= 0 || opts.PageSize > 200 {
		opts.PageSize = 50
	}

	base := r.applyFilters(r.db.WithContext(ctx).Model(&models.Review{}), opts)

	var total int64
	if err := base.Session(&gormLib.Session{}).
		Distinct("author").Count(&total).Error; err != nil {
		return nil, err
	}

	orderDir := "DESC"
	if opts.Order == "asc" {
		orderDir = "ASC"
	}
	sortCol := r.sortColumn(opts.Sort)

	var rows []contributorRow
	err := r.db.WithContext(ctx).Raw(`
		SELECT `+contributorSelectCols+`
		FROM reviews
		WHERE author != '' AND `+baseReviewWhere+`
			`+filterClause(opts)+`
		GROUP BY author
		ORDER BY `+sortCol+` `+orderDir+`, author ASC
		LIMIT ? OFFSET ?
	`, append(filterArgs(opts), opts.PageSize, (opts.Page-1)*opts.PageSize)...).Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	return &models.ContributorListResult{
		Contributors: rowsToSummaries(rows),
		Total:        total,
		Page:         opts.Page,
		PageSize:     opts.PageSize,
	}, nil
}

func (r *ContributorStatsRepo) GetContributor(ctx context.Context, opts models.ContributorStatsOptions) (*models.ContributorDetail, error) {
	var row contributorRow
	err := r.db.WithContext(ctx).Raw(`
		SELECT `+contributorSelectCols+`
		FROM reviews
		WHERE author = ? AND `+baseReviewWhere+`
			`+filterClause(opts)+`
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
	r.db.WithContext(ctx).Raw(`
		SELECT
			COALESCE(MAX(NULLIF(response->'report'->'score'->>'value','')::int), 0),
			COALESCE(MIN(NULLIF(response->'report'->'score'->>'value','')::int), 0),
			COALESCE(
				PERCENTILE_CONT(0.5) WITHIN GROUP (
					ORDER BY NULLIF(response->'report'->'score'->>'value','')::int
				), 0
			),
			COALESCE(AVG(jsonb_array_length(response->'findings')), 0),
			COALESCE(AVG(NULLIF(response->'report'->'score'->>'changed_lines','')::int), 0)
		FROM reviews
		WHERE author = ? AND `+baseReviewWhere+`
	`, opts.Author).Row().Scan(&maxScore, &minScore, &medianScore, &avgFindings, &avgChangedLines)

	type gradeRow struct {
		Grade string `gorm:"column:grade"`
		Count int    `gorm:"column:cnt"`
	}
	var gradeRows []gradeRow
	r.db.WithContext(ctx).Raw(`
		SELECT response->'report'->'score'->>'grade' AS grade, COUNT(*) AS cnt
		FROM reviews
		WHERE author = ? AND `+baseReviewWhere+`
		GROUP BY grade
	`, opts.Author).Scan(&gradeRows)

	gradeDist := make(map[string]int)
	for _, gr := range gradeRows {
		gradeDist[gr.Grade] = gr.Count
	}

	return &models.ContributorDetail{
		ContributorSummary: rowToSummary(row),
		MedianScore:        mathx.Round(medianScore, 2),
		MaxScore:           maxScore,
		MinScore:           minScore,
		GradeDistribution:  gradeDist,
		AvgFindings:        mathx.Round(avgFindings, 2),
		AvgChangedLines:    mathx.Round(avgChangedLines, 2),
	}, nil
}

func (r *ContributorStatsRepo) GetContributorTrend(ctx context.Context, opts models.ContributorStatsOptions) ([]models.TrendPoint, error) {
	var rows []struct {
		Period      string  `gorm:"column:period"`
		AvgScore    float64 `gorm:"column:avg_score"`
		ReviewCount int     `gorm:"column:review_count"`
	}
	err := r.db.WithContext(ctx).Raw(`
		SELECT
			TO_CHAR(date_trunc('week', created_at), 'YYYY-MM-DD') AS period,
			COALESCE(AVG(NULLIF(response->'report'->'score'->>'value','')::int), 0) AS avg_score,
			COUNT(*) AS review_count
		FROM reviews
		WHERE author = ? AND `+baseReviewWhere+`
			`+filterClause(opts)+`
		GROUP BY period
		ORDER BY period ASC
	`, append([]interface{}{opts.Author}, filterArgs(opts)...)...).Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	points := make([]models.TrendPoint, 0, len(rows))
	for _, row := range rows {
		points = append(points, models.TrendPoint{
			Period:      row.Period,
			AvgScore:    mathx.Round(row.AvgScore, 2),
			ReviewCount: row.ReviewCount,
		})
	}
	return points, nil
}

func (r *ContributorStatsRepo) GetContributorCategories(ctx context.Context, opts models.ContributorStatsOptions) ([]models.CategoryStat, error) {
	var rows []struct {
		Category string `gorm:"column:category"`
		Count    int    `gorm:"column:cnt"`
	}
	err := r.db.WithContext(ctx).Raw(`
		SELECT
			finding->>'category' AS category,
			COUNT(*) AS cnt
		FROM reviews, jsonb_array_elements(response->'findings') AS finding
		WHERE author = ? AND `+baseReviewWhere+`
			`+filterClause(opts)+`
		GROUP BY category
		ORDER BY cnt DESC
	`, append([]interface{}{opts.Author}, filterArgs(opts)...)...).Scan(&rows).Error
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
			SELECT provider, repo
			FROM reviews
			WHERE `+baseReviewWhere+`
				`+projectFilterClause(opts)+`
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
	sortCol := r.projectSortColumn(opts.Sort)

	var rows []projectRow
	err = r.db.WithContext(ctx).Raw(`
		SELECT `+projectSelectCols+`
		FROM reviews
		WHERE `+baseReviewWhere+`
			`+projectFilterClause(opts)+`
		GROUP BY provider, repo
		ORDER BY `+sortCol+` `+orderDir+`, provider ASC, repo ASC
		LIMIT ? OFFSET ?
	`, append(projectFilterArgs(opts), opts.PageSize, (opts.Page-1)*opts.PageSize)...).Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	return &models.ProjectListResult{
		Projects: projectRowsToSummaries(rows),
		Total:    total,
		Page:     opts.Page,
		PageSize: opts.PageSize,
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
	r.db.WithContext(ctx).Raw(`
		SELECT
			COALESCE(MAX(NULLIF(response->'report'->'score'->>'value','')::int), 0),
			COALESCE(MIN(NULLIF(response->'report'->'score'->>'value','')::int), 0),
			COALESCE(
				PERCENTILE_CONT(0.5) WITHIN GROUP (
					ORDER BY NULLIF(response->'report'->'score'->>'value','')::int
				), 0
			),
			COALESCE(AVG(jsonb_array_length(response->'findings')), 0),
			COALESCE(AVG(NULLIF(response->'report'->'score'->>'changed_lines','')::int), 0)
		FROM reviews
		WHERE provider = ? AND repo = ? AND `+baseReviewWhere+`
			`+projectFilterClauseWithoutProject(opts)+`
	`, append([]interface{}{opts.Provider, opts.Repo}, projectFilterArgsWithoutProject(opts)...)...).Row().Scan(&maxScore, &minScore, &medianScore, &avgFindings, &avgChangedLines)

	type gradeRow struct {
		Grade string `gorm:"column:grade"`
		Count int    `gorm:"column:cnt"`
	}
	var gradeRows []gradeRow
	r.db.WithContext(ctx).Raw(`
		SELECT response->'report'->'score'->>'grade' AS grade, COUNT(*) AS cnt
		FROM reviews
		WHERE provider = ? AND repo = ? AND `+baseReviewWhere+`
			`+projectFilterClauseWithoutProject(opts)+`
		GROUP BY grade
	`, append([]interface{}{opts.Provider, opts.Repo}, projectFilterArgsWithoutProject(opts)...)...).Scan(&gradeRows)

	gradeDist := make(map[string]int)
	for _, gr := range gradeRows {
		gradeDist[gr.Grade] = gr.Count
	}

	return &models.ProjectDetail{
		ProjectSummary:    projectRowToSummary(row),
		MedianScore:       mathx.Round(medianScore, 2),
		MaxScore:          maxScore,
		MinScore:          minScore,
		GradeDistribution: gradeDist,
		AvgFindings:       mathx.Round(avgFindings, 2),
		AvgChangedLines:   mathx.Round(avgChangedLines, 2),
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
		SELECT
			TO_CHAR(date_trunc('week', created_at), 'YYYY-MM-DD') AS period,
			COALESCE(AVG(NULLIF(response->'report'->'score'->>'value','')::int), 0) AS avg_score,
			COUNT(*) AS review_count,
			COALESCE(SUM(jsonb_array_length(response->'findings')), 0) AS total_findings
		FROM reviews
		WHERE provider = ? AND repo = ? AND `+baseReviewWhere+`
			`+projectFilterClauseWithoutProject(opts)+`
		GROUP BY period
		ORDER BY period ASC
	`, append([]interface{}{opts.Provider, opts.Repo}, projectFilterArgsWithoutProject(opts)...)...).Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	points := make([]models.ProjectTrendPoint, 0, len(rows))
	for _, row := range rows {
		points = append(points, models.ProjectTrendPoint{
			Period:        row.Period,
			AvgScore:      mathx.Round(row.AvgScore, 2),
			ReviewCount:   row.ReviewCount,
			TotalFindings: row.TotalFindings,
		})
	}
	return points, nil
}

func (r *ContributorStatsRepo) GetProjectCategories(ctx context.Context, opts models.ProjectStatsOptions) ([]models.CategoryStat, error) {
	var rows []struct {
		Category string `gorm:"column:category"`
		Count    int    `gorm:"column:cnt"`
	}
	err := r.db.WithContext(ctx).Raw(`
		SELECT
			finding->>'category' AS category,
			COUNT(*) AS cnt
		FROM reviews, jsonb_array_elements(response->'findings') AS finding
		WHERE provider = ? AND repo = ? AND `+baseReviewWhere+`
			`+projectFilterClauseWithoutProject(opts)+`
		GROUP BY category
		ORDER BY cnt DESC
		LIMIT 20
	`, append([]interface{}{opts.Provider, opts.Repo}, projectFilterArgsWithoutProject(opts)...)...).Scan(&rows).Error
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

func (r *ContributorStatsRepo) GetProjectFiles(ctx context.Context, opts models.ProjectStatsOptions) ([]models.FileRiskStat, error) {
	var rows []struct {
		Path          string `gorm:"column:path"`
		Count         int    `gorm:"column:cnt"`
		CriticalCount int    `gorm:"column:critical_count"`
		MajorCount    int    `gorm:"column:major_count"`
	}
	err := r.db.WithContext(ctx).Raw(`
		SELECT
			finding->>'path' AS path,
			COUNT(*) AS cnt,
			COALESCE(SUM(CASE WHEN finding->>'severity' = 'critical' THEN 1 ELSE 0 END), 0) AS critical_count,
			COALESCE(SUM(CASE WHEN finding->>'severity' = 'major' THEN 1 ELSE 0 END), 0) AS major_count
		FROM reviews, jsonb_array_elements(response->'findings') AS finding
		WHERE provider = ? AND repo = ? AND `+baseReviewWhere+`
			AND COALESCE(finding->>'path', '') != ''
			`+projectFilterClauseWithoutProject(opts)+`
		GROUP BY path
		ORDER BY critical_count DESC, major_count DESC, cnt DESC
		LIMIT 20
	`, append([]interface{}{opts.Provider, opts.Repo}, projectFilterArgsWithoutProject(opts)...)...).Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	stats := make([]models.FileRiskStat, 0, len(rows))
	for _, row := range rows {
		stats = append(stats, models.FileRiskStat{
			Path:          row.Path,
			Count:         row.Count,
			CriticalCount: row.CriticalCount,
			MajorCount:    row.MajorCount,
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
		GROUP BY author
		ORDER BY review_count DESC, avg_score DESC, author ASC
		LIMIT ?
	`, append(append([]interface{}{opts.Provider, opts.Repo}, projectFilterArgsWithoutProject(opts)...), limit)...).Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	return rowsToSummaries(rows), nil
}

func filterClause(opts models.ContributorStatsOptions) string {
	clauses := ""
	if opts.Provider != "" {
		clauses += " AND provider = ?"
	}
	if opts.Repo != "" {
		clauses += " AND repo ILIKE ?"
	}
	if !opts.From.IsZero() {
		clauses += " AND created_at >= ?"
	}
	if !opts.To.IsZero() {
		clauses += " AND created_at <= ?"
	}
	return clauses
}

func filterArgs(opts models.ContributorStatsOptions) []interface{} {
	var args []interface{}
	if opts.Provider != "" {
		args = append(args, opts.Provider)
	}
	if opts.Repo != "" {
		args = append(args, "%"+opts.Repo+"%")
	}
	if !opts.From.IsZero() {
		args = append(args, opts.From)
	}
	if !opts.To.IsZero() {
		args = append(args, opts.To)
	}
	return args
}

func projectFilterClause(opts models.ProjectStatsOptions) string {
	clauses := ""
	if opts.Provider != "" {
		clauses += " AND provider = ?"
	}
	if opts.Repo != "" {
		clauses += " AND repo ILIKE ?"
	}
	if !opts.From.IsZero() {
		clauses += " AND created_at >= ?"
	}
	if !opts.To.IsZero() {
		clauses += " AND created_at <= ?"
	}
	return clauses
}

func projectFilterArgs(opts models.ProjectStatsOptions) []interface{} {
	var args []interface{}
	if opts.Provider != "" {
		args = append(args, opts.Provider)
	}
	if opts.Repo != "" {
		args = append(args, "%"+opts.Repo+"%")
	}
	if !opts.From.IsZero() {
		args = append(args, opts.From)
	}
	if !opts.To.IsZero() {
		args = append(args, opts.To)
	}
	return args
}

func projectFilterClauseWithoutProject(opts models.ProjectStatsOptions) string {
	clauses := ""
	if !opts.From.IsZero() {
		clauses += " AND created_at >= ?"
	}
	if !opts.To.IsZero() {
		clauses += " AND created_at <= ?"
	}
	return clauses
}

func projectFilterArgsWithoutProject(opts models.ProjectStatsOptions) []interface{} {
	var args []interface{}
	if !opts.From.IsZero() {
		args = append(args, opts.From)
	}
	if !opts.To.IsZero() {
		args = append(args, opts.To)
	}
	return args
}

func rowToSummary(row contributorRow) models.ContributorSummary {
	return models.ContributorSummary{
		Author:        row.Author,
		AuthorName:    row.AuthorName,
		AuthorAvatar:  row.AuthorAvatar,
		ReviewCount:   row.ReviewCount,
		AvgScore:      mathx.Round(row.AvgScore, 2),
		PassRate:      mathx.Round(row.PassRate*100, 2),
		TotalFindings: row.TotalFindings,
		CriticalCount: row.CriticalCount,
		MajorCount:    row.MajorCount,
		MinorCount:    row.MinorCount,
		InfoCount:     row.InfoCount,
		LastReviewAt:  row.LastReviewAt,
	}
}

func rowsToSummaries(rows []contributorRow) []models.ContributorSummary {
	return slicex.Map(rows, func(row contributorRow) models.ContributorSummary {
		return rowToSummary(row)
	})
}

func projectRowToSummary(row projectRow) models.ProjectSummary {
	return models.ProjectSummary{
		Provider:      row.Provider,
		Repo:          row.Repo,
		ReviewCount:   row.ReviewCount,
		AvgScore:      mathx.Round(row.AvgScore, 2),
		PassRate:      mathx.Round(row.PassRate*100, 2),
		TotalFindings: row.TotalFindings,
		CriticalCount: row.CriticalCount,
		MajorCount:    row.MajorCount,
		MinorCount:    row.MinorCount,
		InfoCount:     row.InfoCount,
		LastReviewAt:  row.LastReviewAt,
	}
}

func projectRowsToSummaries(rows []projectRow) []models.ProjectSummary {
	return slicex.Map(rows, func(row projectRow) models.ProjectSummary {
		return projectRowToSummary(row)
	})
}
