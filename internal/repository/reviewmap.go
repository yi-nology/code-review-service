package repository

import (
	"context"

	"github.com/yi-nology/code-review-service/internal/repository/postgres/models"
)

type CustomProviderRepository interface {
	List(ctx context.Context) ([]models.CustomProvider, error)
	GetByID(ctx context.Context, id uint) (*models.CustomProvider, error)
	GetByName(ctx context.Context, name string) (*models.CustomProvider, error)
	Create(ctx context.Context, p *models.CustomProvider) error
	Update(ctx context.Context, p *models.CustomProvider) error
	Delete(ctx context.Context, id uint) error
}
