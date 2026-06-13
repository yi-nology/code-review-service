package postgres

import (
	"context"
	"errors"

	"review-service/internal/repository/postgres/models"

	gormLib "gorm.io/gorm"
)

type OCRSettingsRepo struct {
	db *gormLib.DB
}

func NewOCRSettingsRepo(db *gormLib.DB) *OCRSettingsRepo {
	return &OCRSettingsRepo{db: db}
}

func (r *OCRSettingsRepo) Get(ctx context.Context) (*models.OCRSettings, error) {
	var settings models.OCRSettings
	err := r.db.WithContext(ctx).
		Order("id DESC").
		First(&settings).Error
	if err != nil {
		if errors.Is(err, gormLib.ErrRecordNotFound) {
			return &models.OCRSettings{
				Enabled:        false,
				Language:       "Chinese",
				Concurrency:    2,
				MaxGitProcs:    4,
				TimeoutMinutes: 10,
			}, nil
		}
		return nil, err
	}
	return &settings, nil
}

func (r *OCRSettingsRepo) Save(ctx context.Context, settings *models.OCRSettings) error {
	var existing models.OCRSettings
	err := r.db.WithContext(ctx).Order("id DESC").First(&existing).Error
	if errors.Is(err, gormLib.ErrRecordNotFound) {
		return r.db.WithContext(ctx).Create(settings).Error
	}
	if err != nil {
		return err
	}

	settings.ID = existing.ID
	return r.db.WithContext(ctx).Save(settings).Error
}
