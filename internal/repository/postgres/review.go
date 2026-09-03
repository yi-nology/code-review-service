package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"
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
	now := time.Now()
	if !review.CreatedAt.IsZero() {
		m.CreatedAt = review.CreatedAt
	} else {
		m.CreatedAt = now
	}
	if !review.UpdatedAt.IsZero() {
		m.UpdatedAt = review.UpdatedAt
	} else {
		m.UpdatedAt = now
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
			// 不存在：创建。domain 时间戳优先，零值则用当前时间（GORM autoCreateTime
			// 对零值 time.Time 不一定填充，显式设值更可靠）。
			now := time.Now()
			if !review.CreatedAt.IsZero() {
				m.CreatedAt = review.CreatedAt
			} else {
				m.CreatedAt = now
			}
			if !review.UpdatedAt.IsZero() {
				m.UpdatedAt = review.UpdatedAt
			} else {
				m.UpdatedAt = now
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
		return nil, wrapNotFound(err)
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
		return nil, wrapNotFound(err)
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
		return nil, wrapNotFound(err)
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
		return db.Order(reviewSortClause(opts.SortBy, opts.SortOrder))
	})
	if err != nil {
		return nil, err
	}
	out := &ekitGorm.PageResponse[domain.Record]{
		Items: make([]domain.Record, len(resp.Items)),
		Total: resp.Total,
	}
	for i, m := range resp.Items {
		out.Items[i] = reviewToDomain(m)
	}
	return out, nil
}

// reviewSortClause 把前端传入的排序字段白名单映射为 SQL ORDER BY 表达式。
// 字段名严格白名单匹配（绝不字符串拼接外部输入），方向仅允许 asc/desc，
// 其余一律回退到默认 created_at DESC，杜绝 ORDER BY 注入。
func reviewSortClause(by, order string) string {
	columns := map[string]string{
		"score":      "score",
		"updated_at": "updated_at",
		"findings":   "COALESCE(jsonb_array_length(response->'findings'), 0)",
	}
	dir := "DESC"
	if strings.EqualFold(order, "asc") {
		dir = "ASC"
	}
	if col, ok := columns[strings.ToLower(strings.TrimSpace(by))]; ok {
		return col + " " + dir
	}
	return "created_at DESC"
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

// MaxRequeueRetries 是单个评审被 RequeueStale 重新入队的最大次数。
// 超过此上限的孤儿任务视为永久失败（status='failed'），不再无谓重排，
// 防止服务频繁重启时持续失败的任务反复涌入队列造成雪崩。
// 5 次对应约 5 次重启/重排周期，足以覆盖临时故障恢复窗口。
const MaxRequeueRetries = 5

func (r *ReviewRepo) RequeueStale(ctx context.Context, before time.Time) (int64, error) {
	var total int64
	err := ekitGorm.InTransactionContext(ctx, r.db, func(tx *gormLib.DB) error {
		// 1. 超过重试上限的孤儿任务直接标记 failed，不再重新入队。
		failed := tx.Model(&models.Review{}).
			Where("status = 'running' AND updated_at < ? AND retry_count >= ?", before, MaxRequeueRetries).
			Updates(map[string]interface{}{
				"status":     "failed",
				"updated_at": time.Now(),
			})
		if failed.Error != nil {
			return failed.Error
		}

		// 2. 未超上限的重新入队，retry_count + 1。
		requeued := tx.Model(&models.Review{}).
			Where("status = 'running' AND updated_at < ? AND retry_count < ?", before, MaxRequeueRetries).
			Updates(map[string]interface{}{
				"status":      "queued",
				"retry_count": gormLib.Expr("retry_count + 1"),
				"updated_at":  time.Now(),
			})
		if requeued.Error != nil {
			return requeued.Error
		}

		total = failed.RowsAffected + requeued.RowsAffected
		return nil
	})
	return total, err
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
		if !opts.From.IsZero() {
			db = db.Where("created_at >= ?", opts.From)
		}
		if !opts.To.IsZero() {
			db = db.Where("created_at < ?", opts.To)
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

	return &domain.AggregateStats{
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
	if err := applyFilters(r.db.WithContext(ctx).Model(&models.Review{})).
		Select(`COUNT(*) AS review_count,
			COALESCE(SUM(CASE WHEN response #>> '{report,ocr,total_tokens}' ~ '^[0-9]+$' THEN (response #>> '{report,ocr,total_tokens}')::bigint ELSE 0 END), 0) AS total_tokens,
			COALESCE(SUM(CASE WHEN response #>> '{report,ocr,input_tokens}' ~ '^[0-9]+$' THEN (response #>> '{report,ocr,input_tokens}')::bigint ELSE 0 END), 0) AS input_tokens,
			COALESCE(SUM(CASE WHEN response #>> '{report,ocr,output_tokens}' ~ '^[0-9]+$' THEN (response #>> '{report,ocr,output_tokens}')::bigint ELSE 0 END), 0) AS output_tokens,
			COALESCE(SUM(CASE WHEN response #>> '{report,ocr,cache_read_tokens}' ~ '^[0-9]+$' THEN (response #>> '{report,ocr,cache_read_tokens}')::bigint ELSE 0 END), 0) AS cache_read_tokens,
			COALESCE(SUM(CASE WHEN response #>> '{report,ocr,cache_write_tokens}' ~ '^[0-9]+$' THEN (response #>> '{report,ocr,cache_write_tokens}')::bigint ELSE 0 END), 0) AS cache_write_tokens,
			COALESCE(SUM(CASE WHEN response #>> '{report,scope,reviewed_files}' ~ '^[0-9]+$' THEN (response #>> '{report,scope,reviewed_files}')::bigint ELSE 0 END), 0) AS total_reviewed_files,
			COALESCE(AVG(CASE WHEN response #>> '{report,ocr,total_tokens}' ~ '^[0-9]+$' THEN (response #>> '{report,ocr,total_tokens}')::double precision END), 0) AS avg_tokens,
			COALESCE(AVG(score), 0) AS avg_score,
			COALESCE(AVG(CASE WHEN response #>> '{report,score,findings_density}' ~ '^[0-9]+(\.[0-9]+)?$' THEN (response #>> '{report,score,findings_density}')::double precision END), 0) AS avg_findings_density`).
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

// MonthlyTokenUsage 返回当前自然月（UTC）所有成功评审的 token 用量汇总。
// 用于预算检查：Calculator 估算成本时按 model 查单价，这里只取 token 总量和按模型分组。
type ModelTokenUsage struct {
	Model        string
	InputTokens  int64
	OutputTokens int64
	CacheRead    int64
	CacheWrite   int64
}

func (r *ReviewRepo) MonthlyTokenUsage(ctx context.Context) ([]ModelTokenUsage, error) {
	type row struct {
		Model        string
		InputTokens  int64
		OutputTokens int64
		CacheRead    int64
		CacheWrite   int64
	}
	var rows []row
	// 按当前自然月聚合，按 model 分组（model 从 response.report.ocr.model 取）。
	query := `SELECT
			COALESCE(NULLIF(response #>> '{report,ocr,model}',''),'unknown') AS model,
			COALESCE((response #>> '{report,ocr,input_tokens}')::bigint, 0) AS input_tokens,
			COALESCE((response #>> '{report,ocr,output_tokens}')::bigint, 0) AS output_tokens,
			COALESCE((response #>> '{report,ocr,cache_read_tokens}')::bigint, 0) AS cache_read,
			COALESCE((response #>> '{report,ocr,cache_write_tokens}')::bigint, 0) AS cache_write
		FROM reviews
		WHERE status = 'success'
			AND deleted_at IS NULL
			AND date_trunc('month', created_at) = date_trunc('month', NOW())`
	if err := r.db.WithContext(ctx).Raw(query).Scan(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]ModelTokenUsage, len(rows))
	for i, r := range rows {
		out[i] = ModelTokenUsage{
			Model: r.Model, InputTokens: r.InputTokens, OutputTokens: r.OutputTokens,
			CacheRead: r.CacheRead, CacheWrite: r.CacheWrite,
		}
	}
	return out, nil
}

// QueueDepth 返回当前队列深度（queued + running 的 review 数），供 Prometheus 指标采集。
func (r *ReviewRepo) QueueDepth(ctx context.Context) (int64, error) {
	var n int64
	err := r.db.WithContext(ctx).Model(&models.Review{}).
		Where("status IN ?", []string{"queued", "running"}).Count(&n).Error
	return n, err
}

// RequeueFromDeadLetter 把一条死信评审（failed 且 retry_count 达上限）重新入队，
// 重置 retry_count 为 0，给它在重启的 worker 上重新执行的机会。
// 仅对 retry_count >= MaxRequeueRetries 的 failed 评审生效，避免误重排正常失败的评审。
func (r *ReviewRepo) RequeueFromDeadLetter(ctx context.Context, reviewID string) error {
	result := r.db.WithContext(ctx).Model(&models.Review{}).
		Where("review_id = ? AND status = 'failed' AND retry_count >= ?", reviewID, MaxRequeueRetries).
		Updates(map[string]interface{}{
			"status":      "queued",
			"retry_count": 0,
			"updated_at":  time.Now(),
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("%w: %s (not a dead-letter review)", repoerr.ErrNotFound, reviewID)
	}
	return nil
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
