package ocr

import (
	"context"

	"github.com/yi-nology/code-review-service/internal/repository/postgres/models"
)

type settingsSource interface {
	Get(ctx context.Context) (*models.OCRSettings, error)
}

type globalRulesSource interface {
	Get(ctx context.Context) (*models.GlobalRules, error)
}
