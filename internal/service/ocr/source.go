package ocr

import (
	"context"

	"github.com/yi-nology/code-review-service/internal/repository/postgres/models"
)

type SettingsSource interface {
	Get(ctx context.Context) (*models.OCRSettings, error)
}

type GlobalRulesSource interface {
	Get(ctx context.Context) (*models.GlobalRules, error)
}
