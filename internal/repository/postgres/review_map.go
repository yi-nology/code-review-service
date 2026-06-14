package postgres

import (
	"context"

	"github.com/yi-nology/code-review-service/internal/repository/postgres/models"

	gormLib "gorm.io/gorm"
)

type ReviewMapRepo struct {
	db *gormLib.DB
}

func NewReviewMapRepo(db *gormLib.DB) *ReviewMapRepo {
	return &ReviewMapRepo{db: db}
}

// Create creates a new review map
func (r *ReviewMapRepo) Create(ctx context.Context, m *models.ReviewMap) error {
	return r.db.WithContext(ctx).Create(m).Error
}

// GetByMapID returns a review map by its map ID
func (r *ReviewMapRepo) GetByMapID(ctx context.Context, mapID string) (*models.ReviewMap, error) {
	var m models.ReviewMap
	err := r.db.WithContext(ctx).Where("map_id = ?", mapID).First(&m).Error
	if err != nil {
		if err == gormLib.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &m, nil
}

// GetByReviewID returns all review maps for a review
func (r *ReviewMapRepo) GetByReviewID(ctx context.Context, reviewID string) ([]models.ReviewMap, error) {
	var maps []models.ReviewMap
	err := r.db.WithContext(ctx).
		Where("review_id = ?", reviewID).
		Order("created_at DESC").
		Find(&maps).Error
	return maps, err
}

// GetBySessionID returns the review map for a session
func (r *ReviewMapRepo) GetBySessionID(ctx context.Context, sessionID string) (*models.ReviewMap, error) {
	var m models.ReviewMap
	err := r.db.WithContext(ctx).Where("session_id = ?", sessionID).First(&m).Error
	if err != nil {
		if err == gormLib.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &m, nil
}

// Update updates a review map
func (r *ReviewMapRepo) Update(ctx context.Context, m *models.ReviewMap) error {
	return r.db.WithContext(ctx).Save(m).Error
}

// Delete deletes a review map
func (r *ReviewMapRepo) Delete(ctx context.Context, mapID string) error {
	return r.db.WithContext(ctx).Where("map_id = ?", mapID).Delete(&models.ReviewMap{}).Error
}

// DeleteByReviewID deletes all review maps for a review
func (r *ReviewMapRepo) DeleteByReviewID(ctx context.Context, reviewID string) error {
	return r.db.WithContext(ctx).Where("review_id = ?", reviewID).Delete(&models.ReviewMap{}).Error
}
