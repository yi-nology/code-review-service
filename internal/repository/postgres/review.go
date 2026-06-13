package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	ekitGorm "github.com/oekit/gormx"
	"github.com/yi-nology/code-review-service/internal/repository/postgres/models"

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
			return nil, nil
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
			return nil, nil
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
			return nil, nil
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
				return fmt.Errorf("review not found: %s", reviewID)
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

	return &models.AggregateStats{
		Running:    int(running),
		GateFailed: int(gateFailed),
		Findings:   int(totalFindings),
	}, nil
}
