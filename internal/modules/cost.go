package modules

import (
	"context"

	"github.com/yi-nology/code-review-service/internal/config"
	"github.com/yi-nology/code-review-service/internal/repository/postgres"
	"github.com/yi-nology/code-review-service/internal/service/cost"

	"go.uber.org/fx"
	gormDB "gorm.io/gorm"
)

// CostModule 装配 LLM 成本估算与预算检查子系统。
var CostModule = fx.Options(
	fx.Provide(NewCostCalculator, NewBudgetChecker),
)

// NewCostCalculator 用主 DB 构造成本估算器。
func NewCostCalculator(db *gormDB.DB) *cost.Calculator {
	return cost.NewCalculator(db)
}

// NewBudgetChecker 用成本估算器 + 配置 + 月度用量查询构造预算检查器。
func NewBudgetChecker(cfg *config.Config, calc *cost.Calculator, db *gormDB.DB) *cost.BudgetChecker {
	reviewRepo := postgres.NewReviewRepo(db)
	return &cost.BudgetChecker{
		Calc: calc,
		Cfg:  cfg.Cost,
		TokenUsageQuery: func(ctx context.Context) ([]cost.ModelUsage, error) {
			usage, err := reviewRepo.MonthlyTokenUsage(ctx)
			if err != nil {
				return nil, err
			}
			out := make([]cost.ModelUsage, len(usage))
			for i, u := range usage {
				out[i] = cost.ModelUsage{
					Model: u.Model, InputTokens: u.InputTokens, OutputTokens: u.OutputTokens,
					CacheRead: u.CacheRead, CacheWrite: u.CacheWrite,
				}
			}
			return out, nil
		},
	}
}
