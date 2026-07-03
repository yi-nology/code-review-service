package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/yi-nology/code-review-service/internal/repository/postgres/models"
	"github.com/yi-nology/code-review-service/internal/repository/repoerr"
	"github.com/yi-nology/code-review-service/internal/service/review/domain"
	ekitGorm "github.com/oekit/gormx" //lint:ignore SA1019 PageResponse 未废弃，上游包级别注释误标
	"github.com/oekit/pkg/pagination"

	gormLib "gorm.io/gorm"
)

type ReviewRepo struct {
	db *gormLib.DB
}

func NewReviewRepo(db *gormLib.DB) *ReviewRepo {
	return &ReviewRepo{db: db}
}

func (r *ReviewRepo) Create(ctx context.Context, review *domain.Record) error {
	m, err := reviewToModel(review)
	if err != nil {
		return err
	}
	if !review.CreatedAt.IsZero() {
		m.CreatedAt = review.CreatedAt
	}
	if !review.UpdatedAt.IsZero() {
		m.UpdatedAt = review.UpdatedAt
	}
	return r.db.WithContext(ctx).Create(&m).Error
}

// Save 按 review_id（业务键）定位现有行的 gorm 自增主键后整体保存。
// domain.Record 不携带 gorm ID，故需先查（同 Session.Update 模式）。
func (r *ReviewRepo) Save(ctx context.Context, review *domain.Record) error {
	m, err := reviewToModel(review)
	if err != nil {
		return err
	}
	var existing models.Review
	if err := r.db.WithContext(ctx).Where("review_id = ?", review.ID).First(&existing).Error; err != nil {
		if errors.Is(err, gormLib.ErrRecordNotFound) {
			// 不存在：创建（保留 domain 时间戳）
			if !review.CreatedAt.IsZero() {
				m.CreatedAt = review.CreatedAt
			}
			if !review.UpdatedAt.IsZero() {
				m.UpdatedAt = review.UpdatedAt
			}
			return r.db.WithContext(ctx).Create(&m).Error
		}
		return err
	}
	m.ID = existing.ID
	m.CreatedAt = existing.CreatedAt
	if !review.UpdatedAt.IsZero() {
		m.UpdatedAt = review.UpdatedAt
	}
	return r.db.WithContext(ctx).Save(&m).Error
}

func (r *ReviewRepo) GetByReviewID(ctx context.Context, reviewID string) (*domain.Record, error) {
	var m models.Review
	err := r.db.WithContext(ctx).
		Where("review_id = ?", reviewID).
		First(&m).Error
	if err != nil {
		if errors.Is(err, gormLib.ErrRecordNotFound) {
			return nil, repoerr.ErrNotFound
		}
		return nil, err
	}
	d := reviewToDomain(m)
	return &d, nil
}

func (r *ReviewRepo) GetByIdempotencyKey(ctx context.Context, key string) (*domain.Record, error) {
	var m models.Review
	err := r.db.WithContext(ctx).
		Where("idempotency_key = ?", key).
		First(&m).Error
	if err != nil {
		if errors.Is(err, gormLib.ErrRecordNotFound) {
			return nil, repoerr.ErrNotFound
		}
		return nil, err
	}
	d := reviewToDomain(m)
	return &d, nil
}

// GetActiveByProviderRepo 返回 (provider, repo) 下处于 queued/running 的最近一条 review。
// pullRequestID / mergeRequestIID 任一非空时把 PR/MR 过滤推到 SQL，避免 service 层 TOCTOU。
func (r *ReviewRepo) GetActiveByProviderRepo(ctx context.Context, provider, repo, pullRequestID, mergeRequestIID string) (*domain.Record, error) {
	var m models.Review
	db := r.db.WithContext(ctx).
		Where("provider = ? AND repo = ? AND status IN ?", provider, repo, []string{"queued", "running"})

	switch {
	case pullRequestID != "":
		db = db.Where("pull_request_id = ?", pullRequestID)
	case mergeRequestIID != "":
		db = db.Where("merge_request_iid = ?", mergeRequestIID)
	}

	err := db.Order("created_at DESC").First(&m).Error
	if err != nil {
		if errors.Is(err, gormLib.ErrRecordNotFound) {
			return nil, repoerr.ErrNotFound
		}
		return nil, err
	}
	d := reviewToDomain(m)
	return &d, nil
}

func (r *ReviewRepo) List(ctx context.Context, opts domain.ListOptions) (*ekitGorm.PageResponse[domain.Record], error) {
	req := pagination.NewPageRequest(opts.Page, opts.PageSize)

	resp, err := ekitGorm.Paginate[models.Review](r.db.WithContext(ctx).Model(&models.Review{}), req, func(db *gormLib.DB) *gormLib.DB {
		if opts.Status != "" {
			db = db.Where("status = ?", opts.Status)
		}
		if opts.GateStatus != "" {
			db = db.Where("response::jsonb->'gate'->>'status' = ?", opts.GateStatus)
		}
		if opts.Provider != "" {
			db = db.Where("provider = ?", opts.Provider)
		}
		if opts.Repo != "" {
			db = db.Where("repo ILIKE ?", "%"+opts.Repo+"%")
		}
		return db.Order("created_at DESC")
	})
	if err != nil {
		return nil, err
	}
	out := &ekitGorm.PageResponse[domain.Record]{
		List:  make([]domain.Record, len(resp.List)),
		Total: resp.Total,
	}
	for i, m := range resp.List {
		out.List[i] = reviewToDomain(m)
	}
	return out, nil
}

func (r *ReviewRepo) ClaimQueued(ctx context.Context, limit int) ([]domain.Record, error) {
	var ms []models.Review

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
		`, limit).Scan(&ms).Error
	})
	if err != nil {
		return nil, err
	}
	out := make([]domain.Record, len(ms))
	for i, m := range ms {
		out[i] = reviewToDomain(m)
	}
	return out, nil
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

// UpdateInTransaction 在事务内把 model 映射为 domain.Record 交给回调，
// 回调改写 domain 字段后映射回 model 保存（#6 keystone：消除 model 指针泄漏）。
func (r *ReviewRepo) UpdateInTransaction(ctx context.Context, reviewID string, fn func(review *domain.Record) error) error {
	return ekitGorm.InTransactionContext(ctx, r.db, func(tx *gormLib.DB) error {
		var m models.Review
		if err := tx.Where("review_id = ?", reviewID).First(&m).Error; err != nil {
			if errors.Is(err, gormLib.ErrRecordNotFound) {
				return fmt.Errorf("%w: %s", repoerr.ErrNotFound, reviewID)
			}
			return err
		}
		rec := reviewToDomain(m)
		if err := fn(&rec); err != nil {
			return err
		}
		updated, err := reviewToModel(&rec)
		if err != nil {
			return err
		}
		updated.ID = m.ID // 保留 gorm 自增主键
		return tx.Save(&updated).Error
	})
}

func (r *ReviewRepo) AggregateStats(ctx context.Context, opts domain.ListOptions) (*domain.AggregateStats, error) {
	applyFilters := func(db *gormLib.DB) *gormLib.DB {
		if opts.Status != "" {
			db = db.Where("status = ?", opts.Status)
		}
		if opts.GateStatus != "" {
			db = db.Where("response::jsonb->'gate'->>'status' = ?", opts.GateStatus)
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

	type tokenSums struct {
		Total     int64
		Input     int64
		Output    int64
		CacheRead int64
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

	return &domain.AggregateStats{
		Running:         int(running),
		GateFailed:      int(gateFailed),
		Findings:        int(totalFindings),
		TotalTokens:     tokens.Total,
		InputTokens:     tokens.Input,
		OutputTokens:    tokens.Output,
		CacheReadTokens: tokens.CacheRead,
		CacheWriteTokens: tokens.CacheWrite,
	}, nil
}

func (r *ReviewRepo) ProjectOCRMetrics(ctx context.Context, opts domain.ProjectStatsOptions) (*domain.ProjectOCRMetrics, error) {
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
			number, number, number, number, number, number, number, number, decimal).
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
	toolCalls := make([]domain.ToolCallStat, 0, len(toolRows))
	for _, row := range toolRows {
		toolCalls = append(toolCalls, domain.ToolCallStat{Tool: row.Tool, Count: row.Count})
	}

	return &domain.ProjectOCRMetrics{
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

func (r *ReviewRepo) ProjectOCRTrend(ctx context.Context, opts domain.ProjectStatsOptions) ([]domain.ProjectOCRTrendPoint, error) {
	clause, args := reviewMetricsFilterClause(opts)
	type trendRow struct {
		Period          string
		ReviewCount     int64
		TotalTokens     int64
		AvgTokens       float64
		AvgElapsedMs    float64
		AvgScore        float64
		FindingsDensity float64
	}
	var rows []trendRow
	query := `SELECT
			to_char(date_trunc('day', created_at), 'YYYY-MM-DD') AS period,
			COUNT(*) AS review_count,
			COALESCE(SUM(CASE WHEN response #>> '{report,ocr,total_tokens}' ~ '^[0-9]+$' THEN (response #>> '{report,ocr,total_tokens}')::bigint ELSE 0 END), 0) AS total_tokens,
			COALESCE(AVG(CASE WHEN response #>> '{report,ocr,total_tokens}' ~ '^[0-9]+$' THEN (response #>> '{report,ocr,total_tokens}')::double precision END), 0) AS avg_tokens,
			COALESCE(AVG(CASE WHEN response #>> '{report,ocr,elapsed}' ~ '^[0-9]+(\.[0-9]+)?s$' THEN replace(response #>> '{report,ocr,elapsed}', 's', '')::double precision * 1000 END), 0) AS avg_elapsed_ms,
			COALESCE(AVG(score), 0) AS avg_score,
			COALESCE(AVG(CASE WHEN response #>> '{report,score,findings_density}' ~ '^[0-9]+(\.[0-9]+)?$' THEN (response #>> '{report,score,findings_density}')::double precision END), 0) AS findings_density
		FROM reviews
		WHERE deleted_at IS NULL AND status = 'success'` + clause + `
		GROUP BY date_trunc('day', created_at)
		ORDER BY period ASC`
	if err := r.db.WithContext(ctx).Raw(query, args...).Scan(&rows).Error; err != nil {
		return nil, err
	}
	points := make([]domain.ProjectOCRTrendPoint, 0, len(rows))
	for _, row := range rows {
		points = append(points, domain.ProjectOCRTrendPoint{
			Period:          row.Period,
			ReviewCount:     row.ReviewCount,
			TotalTokens:     row.TotalTokens,
			AvgTokens:       row.AvgTokens,
			AvgElapsedMs:    row.AvgElapsedMs,
			AvgScore:        row.AvgScore,
			FindingsDensity: row.FindingsDensity,
		})
	}
	return points, nil
}

func reviewMetricsFilterClause(opts domain.ProjectStatsOptions) (string, []any) {
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
