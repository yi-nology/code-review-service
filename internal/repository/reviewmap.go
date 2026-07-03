package repository

import (
	"context"

	"github.com/yi-nology/code-review-service/internal/domain/llmprovider"
)

// CustomProviderRepository 已迁移为 domain 类型（#6）：不再依赖 postgres/models。
type CustomProviderRepository interface {
	List(ctx context.Context) ([]llmprovider.Custom, error)
	GetByID(ctx context.Context, id uint) (*llmprovider.Custom, error)
	GetByName(ctx context.Context, name string) (*llmprovider.Custom, error)
	Create(ctx context.Context, p *llmprovider.Custom) error
	Update(ctx context.Context, p *llmprovider.Custom) error
	Delete(ctx context.Context, id uint) error
}
