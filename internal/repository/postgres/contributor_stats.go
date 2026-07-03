package postgres

import (
	"time"

	"github.com/yi-nology/code-review-service/internal/domain/stats"
	"github.com/yi-nology/code-review-service/internal/service/review/domain"
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

func (r *ContributorStatsRepo) applyFilters(db *gormLib.DB, opts stats.ContributorStatsOptions) *gormLib.DB {
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

func rowToSummary(row contributorRow) stats.ContributorSummary {
	return stats.ContributorSummary{
		Author: row.Author, AuthorName: row.AuthorName, AuthorAvatar: row.AuthorAvatar,
		ReviewCount: row.ReviewCount, AvgScore: mathx.Round(row.AvgScore, 2),
		PassRate: mathx.Round(row.PassRate*100, 2), TotalFindings: row.TotalFindings,
		CriticalCount: row.CriticalCount, MajorCount: row.MajorCount,
		MinorCount: row.MinorCount, InfoCount: row.InfoCount, LastReviewAt: row.LastReviewAt,
	}
}

func rowsToSummaries(rows []contributorRow) []stats.ContributorSummary {
	return slicex.Map(rows, rowToSummary)
}

func projectRowToSummary(row projectRow) stats.ProjectSummary {
	return stats.ProjectSummary{
		Provider: row.Provider, Repo: row.Repo,
		ReviewCount: row.ReviewCount, AvgScore: mathx.Round(row.AvgScore, 2),
		PassRate: mathx.Round(row.PassRate*100, 2), TotalFindings: row.TotalFindings,
		CriticalCount: row.CriticalCount, MajorCount: row.MajorCount,
		MinorCount: row.MinorCount, InfoCount: row.InfoCount, LastReviewAt: row.LastReviewAt,
	}
}

func projectRowsToSummaries(rows []projectRow) []stats.ProjectSummary {
	return slicex.Map(rows, projectRowToSummary)
}

func filterClause(opts stats.ContributorStatsOptions) string {
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

func filterArgs(opts stats.ContributorStatsOptions) []interface{} {
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

func projectFilterClause(opts domain.ProjectStatsOptions) string {
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

func projectFilterArgs(opts domain.ProjectStatsOptions) []interface{} {
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

func projectFilterClauseWithoutProject(opts domain.ProjectStatsOptions) string {
	clauses := ""
	if !opts.From.IsZero() {
		clauses += " AND created_at >= ?"
	}
	if !opts.To.IsZero() {
		clauses += " AND created_at <= ?"
	}
	return clauses
}

func projectFilterArgsWithoutProject(opts domain.ProjectStatsOptions) []interface{} {
	var args []interface{}
	if !opts.From.IsZero() {
		args = append(args, opts.From)
	}
	if !opts.To.IsZero() {
		args = append(args, opts.To)
	}
	return args
}
