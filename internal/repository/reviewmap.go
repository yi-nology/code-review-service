package repository

import (
	"context"

	"github.com/yi-nology/code-review-service/internal/repository/postgres/models"
)

type ReviewMapRepository interface {
	Create(ctx context.Context, m *models.ReviewMap) error
	GetByMapID(ctx context.Context, mapID string) (*models.ReviewMap, error)
	GetByReviewID(ctx context.Context, reviewID string) ([]models.ReviewMap, error)
	GetBySessionID(ctx context.Context, sessionID string) (*models.ReviewMap, error)
	Update(ctx context.Context, m *models.ReviewMap) error
	Delete(ctx context.Context, mapID string) error
	DeleteByReviewID(ctx context.Context, reviewID string) error
	UpdateFileReviewed(ctx context.Context, mapID, filePath string, reviewed bool) error
	UpdateSectionFilesReviewed(ctx context.Context, mapID, sectionID string, reviewed bool) error
}

type CustomProviderRepository interface {
	List(ctx context.Context) ([]models.CustomProvider, error)
	GetByID(ctx context.Context, id uint) (*models.CustomProvider, error)
	GetByName(ctx context.Context, name string) (*models.CustomProvider, error)
	Create(ctx context.Context, p *models.CustomProvider) error
	Update(ctx context.Context, p *models.CustomProvider) error
	Delete(ctx context.Context, id uint) error
}
