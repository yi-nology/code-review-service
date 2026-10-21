package modules

import (
	"github.com/yi-nology/code-review-service/internal/repository"
	"github.com/yi-nology/code-review-service/internal/repository/postgres"
	"github.com/yi-nology/code-review-service/internal/service/session"

	"go.uber.org/fx"
	"gorm.io/gorm"
)

var RepositoryModule = fx.Options(
	fx.Provide(postgres.NewRepoContainer),
	fx.Provide(NewSessionRepository),
	fx.Provide(NewPricingRepository),
	fx.Provide(session.NewSessionService),
)

func NewSessionRepository(db *gorm.DB) repository.SessionRepository {
	return postgres.NewPostgresSessionRepository(db)
}

// NewPricingRepository 提供模型单价仓储（成本估算子系统用）。
func NewPricingRepository(db *gorm.DB) repository.PricingRepository {
	return postgres.NewPricingRepo(db)
}
