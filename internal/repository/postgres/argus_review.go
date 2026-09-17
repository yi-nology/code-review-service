package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/yi-nology/code-review-service/internal/domain/argusingest"
	"github.com/yi-nology/code-review-service/internal/repository/postgres/models"

	gormLib "gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ArgusReviewRepo Argus 上报审查结果的 postgres 实现。
// 事务语义：任务按业务键 (platform,owner,repo,number,head_sha) upsert，
// 意见整组替换（先删后插）——重报同 SHA 时意见集合以最新一次为准。
type ArgusReviewRepo struct {
	db *gormLib.DB
}

func NewArgusReviewRepo(db *gormLib.DB) *ArgusReviewRepo {
	return &ArgusReviewRepo{db: db}
}

func (r *ArgusReviewRepo) UpsertReview(ctx context.Context, rev *argusingest.Review) (uint, error) {
	m := argusReviewToModel(rev)
	err := r.db.WithContext(ctx).Transaction(func(tx *gormLib.DB) error {
		if err := tx.Clauses(clause.OnConflict{
			Columns: []clause.Column{
				{Name: "platform"}, {Name: "owner"}, {Name: "repo"},
				{Name: "number"}, {Name: "head_sha"},
			},
			DoUpdates: clause.AssignmentColumns([]string{
				"argus_task_id", "title", "base_branch", "verdict", "error_message",
				"findings_total", "findings_high", "findings_medium", "findings_low",
				"plugins", "duration_ms", "token_usage", "occurred_at", "updated_at",
			}),
		}).Create(&m).Error; err != nil {
			return err
		}
		var id uint
		if err := tx.Model(&models.ArgusReview{}).Select("id").
			Where("platform = ? AND owner = ? AND repo = ? AND number = ? AND head_sha = ?",
				m.Platform, m.Owner, m.Repo, m.Number, m.HeadSHA).
			Scan(&id).Error; err != nil {
			return err
		}
		if id == 0 {
			return errors.New("argusingest: upsert 后未取回任务 ID")
		}
		// 重报同 SHA 时意见整组替换，但已确认的误报抑制标记按 fp 继承——
		// argus 侧的抑制是持久的，本地标记不应因一次重报被清空。
		var suppressedFPs []string
		if err := tx.Model(&models.ArgusFinding{}).
			Where("review_id = ? AND suppress_state = 'suppressed' AND fp <> ''", id).
			Pluck("fp", &suppressedFPs).Error; err != nil {
			return err
		}
		suppressed := make(map[string]struct{}, len(suppressedFPs))
		for _, fp := range suppressedFPs {
			suppressed[fp] = struct{}{}
		}
		if err := tx.Where("review_id = ?", id).Delete(&models.ArgusFinding{}).Error; err != nil {
			return err
		}
		if len(rev.Findings) > 0 {
			if err := tx.Create(argusFindingsToModels(id, rev.Findings, suppressed)).Error; err != nil {
				return err
			}
		}
		m.ID = id
		return nil
	})
	if err != nil {
		return 0, err
	}
	return m.ID, nil
}

func (r *ArgusReviewRepo) GetReview(ctx context.Context, id uint) (*argusingest.Review, error) {
	var m models.ArgusReview
	if err := r.db.WithContext(ctx).First(&m, id).Error; err != nil {
		return nil, err
	}
	rev := argusModelToReview(m)
	var fms []models.ArgusFinding
	if err := r.db.WithContext(ctx).
		Where("review_id = ?", id).
		Order("file ASC, line ASC, id ASC").
		Find(&fms).Error; err != nil {
		return nil, err
	}
	rev.Findings = make([]argusingest.Finding, 0, len(fms))
	for _, fm := range fms {
		rev.Findings = append(rev.Findings, argusFindingToDomain(fm))
	}
	return rev, nil
}

func (r *ArgusReviewRepo) ListReviews(ctx context.Context, filter argusingest.ListFilter) ([]argusingest.Review, int64, error) {
	q := r.db.WithContext(ctx).Model(&models.ArgusReview{})
	if filter.Repo != "" {
		q = q.Where("repo = ? OR owner || '/' || repo = ?", filter.Repo, filter.Repo)
	}
	if filter.Platform != "" {
		q = q.Where("platform = ?", filter.Platform)
	}
	switch filter.Severity {
	case argusingest.SeverityHigh:
		q = q.Where("findings_high > 0")
	case argusingest.SeverityMedium:
		q = q.Where("findings_medium > 0")
	case argusingest.SeverityLow:
		q = q.Where("findings_low > 0")
	}
	if filter.Since != nil {
		q = q.Where("occurred_at >= ?", *filter.Since)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	page, size := filter.Page, filter.PageSize
	if page <= 0 {
		page = 1
	}
	if size <= 0 || size > 200 {
		size = 20
	}
	var items []models.ArgusReview
	if err := q.Order("occurred_at DESC, id DESC").
		Limit(size).Offset((page - 1) * size).
		Find(&items).Error; err != nil {
		return nil, 0, err
	}
	out := make([]argusingest.Review, 0, len(items))
	for _, m := range items {
		out = append(out, *argusModelToReview(m))
	}
	return out, total, nil
}

// SetFindingSuppressState 标记意见的抑制回流状态。
// 只允许两态：”（撤销标记用）与 'suppressed'。
func (r *ArgusReviewRepo) SetFindingSuppressState(ctx context.Context, reviewID uint, fp, state string) error {
	if state != "" && state != "suppressed" {
		return errors.New("argusingest: 非法 suppress_state")
	}
	return r.db.WithContext(ctx).Model(&models.ArgusFinding{}).
		Where("review_id = ? AND fp = ?", reviewID, fp).
		Update("suppress_state", state).Error
}

// ---------- domain ↔ model 映射 ----------

func argusReviewToModel(rev *argusingest.Review) models.ArgusReview {
	plugins, _ := json.Marshal(rev.Plugins)
	return models.ArgusReview{
		ID: rev.ID, ArgusTaskID: rev.ArgusTaskID,
		Platform: rev.Platform, Owner: rev.Owner, Repo: rev.Repo,
		Number: rev.Number, Title: rev.Title, HeadSHA: rev.HeadSHA,
		BaseBranch: rev.BaseBranch, Verdict: rev.Verdict,
		FindingsTotal: rev.FindingsTotal, FindingsHigh: rev.FindingsHigh,
		FindingsMedium: rev.FindingsMedium, FindingsLow: rev.FindingsLow,
		Plugins: string(plugins), DurationMS: rev.DurationMS, TokenUsage: rev.TokenUsage,
		ErrorMessage: rev.ErrorMessage,
		OccurredAt:   rev.OccurredAt,
	}
}

func argusModelToReview(m models.ArgusReview) *argusingest.Review {
	rev := &argusingest.Review{
		ID: m.ID, ArgusTaskID: m.ArgusTaskID,
		Platform: m.Platform, Owner: m.Owner, Repo: m.Repo,
		Number: m.Number, Title: m.Title, HeadSHA: m.HeadSHA,
		BaseBranch: m.BaseBranch, Verdict: m.Verdict,
		FindingsTotal: m.FindingsTotal, FindingsHigh: m.FindingsHigh,
		FindingsMedium: m.FindingsMedium, FindingsLow: m.FindingsLow,
		DurationMS: m.DurationMS, TokenUsage: m.TokenUsage,
		ErrorMessage: m.ErrorMessage,
		OccurredAt:   m.OccurredAt,
	}
	_ = json.Unmarshal([]byte(m.Plugins), &rev.Plugins)
	return rev
}

func argusFindingsToModels(reviewID uint, fs []argusingest.Finding, suppressed map[string]struct{}) []models.ArgusFinding {
	out := make([]models.ArgusFinding, 0, len(fs))
	now := time.Now()
	for _, f := range fs {
		m := models.ArgusFinding{
			ReviewID: reviewID, FP: f.FP,
			File: f.File, Line: f.Line, Severity: f.Severity,
			Comment: f.Comment, SuggestedFix: f.SuggestedFix, By: f.By,
			CreatedAt: now,
		}
		if _, ok := suppressed[f.FP]; ok {
			m.SuppressState = "suppressed"
		}
		out = append(out, m)
	}
	return out
}

func argusFindingToDomain(m models.ArgusFinding) argusingest.Finding {
	return argusingest.Finding{
		FP: m.FP, File: m.File, Line: m.Line, Severity: m.Severity,
		Comment: m.Comment, SuggestedFix: m.SuggestedFix, By: m.By,
		SuppressState: m.SuppressState,
	}
}
