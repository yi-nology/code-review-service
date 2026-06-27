package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/yi-nology/code-review-service/internal/repository/postgres/models"
	"github.com/yi-nology/code-review-service/internal/repository/repoerr"
	ekitGorm "github.com/oekit/gormx"

	gormLib "gorm.io/gorm"
)

type ReviewRepo struct {
	db *gormLib.DB
}

func NewReviewRepo(db *gormLib.DB) *ReviewRepo {
	return &ReviewRepo{db: db}
}

func (r *ReviewRepo) Create(ctx context.Context, review *models.Review) error {
	return r.db.WithContext(ctx).Create(review).Error
}

func (r *ReviewRepo) Save(ctx context.Context, review *models.Review) error {
	return r.db.WithContext(ctx).Save(review).Error
}

func (r *ReviewRepo) GetByReviewID(ctx context.Context, reviewID string) (*models.Review, error) {
	var review models.Review
	err := r.db.WithContext(ctx).
		Where("review_id = ?", reviewID).
		First(&review).Error
	if err != nil {
		if errors.Is(err, gormLib.ErrRecordNotFound) {
			return nil, repoerr.ErrNotFound
		}
		return nil, err
	}
	return &review, nil
}

func (r *ReviewRepo) GetByidempotencyKey(ctx context.Context, key string) (*models.Review, error) {
	var review models.Review
	err := r.db.WithContext(ctx).
		Where("idempotency_key = ?", key).
		First(&review).Error
	if err != nil {
		if errors.Is(err, gormLib.ErrRecordNotFound) {
			return nil, repoerr.ErrNotFound
		}
		return nil, err
	}
	return &review, nil
}

func (r *ReviewRepo) GetActiveByProviderRepo(ctx context.Context, provider, repo string) (*models.Review, error) {
	var review models.Review
	err := r.db.WithContext(ctx).
		Where("provider = ? AND repo = ? AND status IN ?", provider, repo, []string{"queued", "running"}).
		Order("created_at DESC").
		First(&review).Error
	if err != nil {
		if errors.Is(err, gormLib.ErrRecordNotFound) {
			return nil, repoerr.ErrNotFound
		}
		return nil, err
	}
	return &review, nil
}

func (r *ReviewRepo) List(ctx context.Context, opts models.ListOptions) (*ekitGorm.PageResponse[models.Review], error) {
	req := ekitGorm.NewPageRequest(opts.Page, opts.PageSize)

	return ekitGorm.Paginate[models.Review](r.db.WithContext(ctx).Model(&models.Review{}), req, func(db *gormLib.DB) *gormLib.DB {
		if opts.Status != "" {
			db = db.Where("status = ?", opts.Status)
		}
		if opts.Provider != "" {
			db = db.Where("provider = ?", opts.Provider)
		}
		if opts.Repo != "" {
			db = db.Where("repo ILIKE ?", "%"+opts.Repo+"%")
		}
		return db.Order("created_at DESC")
	})
}

func (r *ReviewRepo) ClaimQueued(ctx context.Context, limit int) ([]models.Review, error) {
	var reviews []models.Review

	err := ekitGorm.InTransactionContext(ctx, r.db, func(tx *gormLib.DB) error {
		return tx.Raw(`
			WITH claimed AS (
				SELECT id FROM reviews
				WHERE status = 'queued' AND deleted_at IS NULL
				ORDER BY created_at
				FOR UPDATE SKIP LOCKED
				LIMIT ?
			)
			UPDATE reviews AS r
			SET status = 'running', updated_at = NOW()
			FROM claimed
			WHERE r.id = claimed.id
			RETURNING r.*
		`, limit).Scan(&reviews).Error
	})

	return reviews, err
}

func (r *ReviewRepo) RequeueStale(ctx context.Context, before time.Time) (int64, error) {
	result := r.db.WithContext(ctx).
		Model(&models.Review{}).
		Where("status = 'running' AND updated_at < ?", before).
		Updates(map[string]interface{}{
			"status":     "queued",
			"updated_at": time.Now(),
		})
	return result.RowsAffected, result.Error
}

func (r *ReviewRepo) UpdateInTransaction(ctx context.Context, reviewID string, fn func(review *models.Review) error) error {
	return ekitGorm.InTransactionContext(ctx, r.db, func(tx *gormLib.DB) error {
		var review models.Review
		if err := tx.Where("review_id = ?", reviewID).First(&review).Error; err != nil {
			if errors.Is(err, gormLib.ErrRecordNotFound) {
				return fmt.Errorf("%w: %s", repoerr.ErrNotFound, reviewID)
			}
			return err
		}
		if err := fn(&review); err != nil {
			return err
		}
		return tx.Save(&review).Error
	})
}

func (r *ReviewRepo) AggregateStats(ctx context.Context, opts models.ListOptions) (*models.AggregateStats, error) {
	applyFilters := func(db *gormLib.DB) *gormLib.DB {
		if opts.Status != "" {
			db = db.Where("status = ?", opts.Status)
		}
		if opts.Provider != "" {
			db = db.Where("provider = ?", opts.Provider)
		}
		if opts.Repo != "" {
			db = db.Where("repo ILIKE ?", "%"+opts.Repo+"%")
		}
		return db
	}

	var running int64
	if err := applyFilters(r.db.WithContext(ctx).Model(&models.Review{})).
		Where("status IN ?", []string{"running", "queued"}).Count(&running).Error; err != nil {
		return nil, err
	}

	var gateFailed int64
	if err := applyFilters(r.db.WithContext(ctx).Model(&models.Review{})).
		Where("response::jsonb->'gate'->>'status' = 'failed'").Count(&gateFailed).Error; err != nil {
		return nil, err
	}

	var totalFindings int64
	if err := applyFilters(r.db.WithContext(ctx).Model(&models.Review{})).
		Select("COALESCE(SUM(jsonb_array_length(response->'findings')), 0)").
		Scan(&totalFindings).Error; err != nil {
		return nil, err
	}

	// Token 用量聚合（#20）：从 response jsonb 的 report.ocr 路径 SUM，
	// 仅统计成功评审（失败评审无 token 数据）。
	type tokenSums struct {
		Total      int64
		Input      int64
		Output     int64
		CacheRead  int64
		CacheWrite int64
	}
	var tokens tokenSums
	if err := applyFilters(r.db.WithContext(ctx).Model(&models.Review{})).
		Where("status = ?", "success").
		Select(`COALESCE(SUM((response->'report'->'ocr'->>'total_tokens')::bigint), 0) AS total,
			COALESCE(SUM((response->'report'->'ocr'->>'input_tokens')::bigint), 0) AS input,
			COALESCE(SUM((response->'report'->'ocr'->>'output_tokens')::bigint), 0) AS output,
			COALESCE(SUM((response->'report'->'ocr'->>'cache_read_tokens')::bigint), 0) AS cache_read,
			COALESCE(SUM((response->'report'->'ocr'->>'cache_write_tokens')::bigint), 0) AS cache_write`).
		Scan(&tokens).Error; err != nil {
		return nil, err
	}

	return &models.AggregateStats{
		Running:          int(running),
		GateFailed:       int(gateFailed),
		Findings:         int(totalFindings),
		TotalTokens:      tokens.Total,
		InputTokens:      tokens.Input,
		OutputTokens:     tokens.Output,
		CacheReadTokens:  tokens.CacheRead,
		CacheWriteTokens: tokens.CacheWrite,
	}, nil
}

func (r *ReviewRepo) ProjectOCRMetrics(ctx context.Context, opts models.ProjectStatsOptions) (*models.ProjectOCRMetrics, error) {
	applyFilters := func(db *gormLib.DB) *gormLib.DB {
		db = db.Where("status = ?", "success")
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

	type summaryRow struct {
		ReviewCount        int64
		TotalTokens        int64
		InputTokens        int64
		OutputTokens       int64
		CacheReadTokens    int64
		CacheWriteTokens   int64
		TotalReviewedFiles int64
		AvgTokens          float64
		AvgScore           float64
		AvgFindingsDensity float64
	}
	var summary summaryRow
	number := `^[0-9]+$`
	decimal := `^[0-9]+(\.[0-9]+)?$`
	if err := applyFilters(r.db.WithContext(ctx).Model(&models.Review{})).
		Select(`COUNT(*) AS review_count,
			COALESCE(SUM(CASE WHEN response #>> '{report,ocr,total_tokens}' ~ ? THEN (response #>> '{report,ocr,total_tokens}')::bigint ELSE 0 END), 0) AS total_tokens,
			COALESCE(SUM(CASE WHEN response #>> '{report,ocr,input_tokens}' ~ ? THEN (response #>> '{report,ocr,input_tokens}')::bigint ELSE 0 END), 0) AS input_tokens,
			COALESCE(SUM(CASE WHEN response #>> '{report,ocr,output_tokens}' ~ ? THEN (response #>> '{report,ocr,output_tokens}')::bigint ELSE 0 END), 0) AS output_tokens,
			COALESCE(SUM(CASE WHEN response #>> '{report,ocr,cache_read_tokens}' ~ ? THEN (response #>> '{report,ocr,cache_read_tokens}')::bigint ELSE 0 END), 0) AS cache_read_tokens,
			COALESCE(SUM(CASE WHEN response #>> '{report,ocr,cache_write_tokens}' ~ ? THEN (response #>> '{report,ocr,cache_write_tokens}')::bigint ELSE 0 END), 0) AS cache_write_tokens,
			COALESCE(SUM(CASE WHEN response #>> '{report,scope,reviewed_files}' ~ ? THEN (response #>> '{report,scope,reviewed_files}')::bigint ELSE 0 END), 0) AS total_reviewed_files,
			COALESCE(AVG(CASE WHEN response #>> '{report,ocr,total_tokens}' ~ ? THEN (response #>> '{report,ocr,total_tokens}')::double precision END), 0) AS avg_tokens,
			COALESCE(AVG(score), 0) AS avg_score,
			COALESCE(AVG(CASE WHEN response #>> '{report,score,findings_density}' ~ ? THEN (response #>> '{report,score,findings_density}')::double precision END), 0) AS avg_findings_density`,
			number, number, number, number, number, number, number, decimal).
		Scan(&summary).Error; err != nil {
		return nil, err
	}

	type gradeRow struct {
		Grade string
		Count int64
	}
	var gradeRows []gradeRow
	if err := applyFilters(r.db.WithContext(ctx).Model(&models.Review{})).
		Select("COALESCE(NULLIF(grade, ''), 'unknown') AS grade, COUNT(*) AS count").
		Group("COALESCE(NULLIF(grade, ''), 'unknown')").
		Scan(&gradeRows).Error; err != nil {
		return nil, err
	}
	gradeDistribution := make(map[string]int64, len(gradeRows))
	for _, row := range gradeRows {
		gradeDistribution[row.Grade] = row.Count
	}

	clause, args := reviewMetricsFilterClause(opts)
	type toolRow struct {
		Tool  string
		Count int64
	}
	var toolRows []toolRow
	query := `SELECT calls.key AS tool, COALESCE(SUM(calls.value::bigint), 0) AS count
		FROM reviews
		CROSS JOIN LATERAL jsonb_each_text(COALESCE(response #> '{report,ocr,tool_calls,by_tool}', '{}'::jsonb)) AS calls(key, value)
		WHERE deleted_at IS NULL AND status = 'success' AND calls.value ~ '^[0-9]+$'` + clause + `
		GROUP BY calls.key
		ORDER BY count DESC`
	if err := r.db.WithContext(ctx).Raw(query, args...).Scan(&toolRows).Error; err != nil {
		return nil, err
	}
	toolCalls := make([]models.ToolCallStat, 0, len(toolRows))
	for _, row := range toolRows {
		toolCalls = append(toolCalls, models.ToolCallStat{Tool: row.Tool, Count: row.Count})
	}

	return &models.ProjectOCRMetrics{
		ReviewCount:        summary.ReviewCount,
		TotalTokens:        summary.TotalTokens,
		InputTokens:        summary.InputTokens,
		OutputTokens:       summary.OutputTokens,
		CacheReadTokens:    summary.CacheReadTokens,
		CacheWriteTokens:   summary.CacheWriteTokens,
		TotalReviewedFiles: summary.TotalReviewedFiles,
		AvgTokens:          summary.AvgTokens,
		AvgScore:           summary.AvgScore,
		AvgFindingsDensity: summary.AvgFindingsDensity,
		GradeDistribution:  gradeDistribution,
		ToolCalls:          toolCalls,
	}, nil
}

func reviewMetricsFilterClause(opts models.ProjectStatsOptions) (string, []any) {
	var clause string
	var args []any
	if opts.Provider != "" {
		clause += " AND provider = ?"
		args = append(args, opts.Provider)
	}
	if opts.Repo != "" {
		clause += " AND repo ILIKE ?"
		args = append(args, "%"+opts.Repo+"%")
	}
	if !opts.From.IsZero() {
		clause += " AND created_at >= ?"
		args = append(args, opts.From)
	}
	if !opts.To.IsZero() {
		clause += " AND created_at <= ?"
		args = append(args, opts.To)
	}
	return clause, args
}
