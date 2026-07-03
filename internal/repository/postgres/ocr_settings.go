package postgres

import (
	"context"
	"errors"
	"strings"

	"github.com/yi-nology/code-review-service/internal/domain/ocrsettings"
	"github.com/yi-nology/code-review-service/internal/repository/postgres/models"

	gormLib "gorm.io/gorm"
)

type OCRSettingsDefaults struct {
	Enabled         bool
	Binary          string
	ContextWindow   int
	MaxOutputTokens int
}

type OCRSettingsRepo struct {
	db       *gormLib.DB
	defaults OCRSettingsDefaults
}

func NewOCRSettingsRepo(db *gormLib.DB, defaults ...OCRSettingsDefaults) *OCRSettingsRepo {
	repo := &OCRSettingsRepo{db: db}
	if len(defaults) > 0 {
		repo.defaults = defaults[0]
	}
	return repo
}

func (r *OCRSettingsRepo) Get(ctx context.Context) (*ocrsettings.Settings, error) {
	var settings models.OCRSettings
	err := r.db.WithContext(ctx).
		Order("id DESC").
		First(&settings).Error
	if err != nil {
		if errors.Is(err, gormLib.ErrRecordNotFound) {
			return r.defaultSettings(), nil
		}
		return nil, err
	}
	d := ocrSettingsToDomain(settings)
	return &d, nil
}

func (r *OCRSettingsRepo) defaultSettings() *ocrsettings.Settings {
	binary := strings.TrimSpace(r.defaults.Binary)
	if binary == "" {
		binary = "ocr"
	}
	return &ocrsettings.Settings{
		Enabled:         r.defaults.Enabled,
		Binary:          binary,
		Audience:        "human",
		DefaultSeverity: "major",
		Language:        "Chinese",
		Concurrency:     2,
		MaxGitProcs:     4,
		TimeoutMinutes:  10,
		ReasoningMode:   "light",
		ContextWindow:   int64(r.defaults.ContextWindow),
		MaxOutputTokens: int64(r.defaults.MaxOutputTokens),
	}
}

func (r *OCRSettingsRepo) Save(ctx context.Context, settings *ocrsettings.Settings) error {
	m := ocrSettingsToModel(settings)
	return r.db.WithContext(ctx).Transaction(func(tx *gormLib.DB) error {
		var existing models.OCRSettings
		err := tx.Order("id DESC").First(&existing).Error
		if errors.Is(err, gormLib.ErrRecordNotFound) {
			return tx.Create(&m).Error
		}
		if err != nil {
			return err
		}
		m.ID = existing.ID
		return tx.Save(&m).Error
	})
}
