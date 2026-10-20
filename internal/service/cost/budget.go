package cost

import (
	"context"
	"fmt"

	"github.com/oekit/logx"
	"github.com/yi-nology/code-review-service/internal/config"
)

// BudgetChecker 在评审执行前检查月度成本预算是否超限。
// 超额时根据配置行为决定：warn（仅告警日志）/ block（拒绝新评审）。
type BudgetChecker struct {
	Calc            *Calculator
	Cfg             config.CostConfig
	TokenUsageQuery func(ctx context.Context) ([]ModelUsage, error)
}

// ModelUsage 是单个模型的月度 token 用量（供预算检查聚合成本）。
type ModelUsage struct {
	Model        string
	InputTokens  int64
	OutputTokens int64
	CacheRead    int64
	CacheWrite   int64
}

// BudgetStatus 是预算检查结果。
type BudgetStatus struct {
	Exceeded bool
	Used     float64 // 本月已用成本（美元）
	Budget   float64 // 预算上限
}

// CheckBlock 在配置为 block 模式且超额时返回 error（拒绝评审）。
// warn 模式或未超额时返回 nil。
func (b *BudgetChecker) CheckBlock(ctx context.Context) error {
	if b == nil || b.Cfg.MonthlyBudgetUSD <= 0 {
		return nil
	}
	status := b.check(ctx)
	if status.Exceeded {
		msg := fmt.Sprintf("月度 LLM 成本预算超限：已用 $%.2f / 预算 $%.2f", status.Used, status.Budget)
		if b.Cfg.ReviewAction == "block" {
			return fmt.Errorf("budget_exceeded: %s", msg)
		}
		logx.CtxWarnf(ctx, "[cost] %s（warn 模式，不阻断）", msg)
	}
	return nil
}

// check 计算本月累计成本并判断是否超额。
func (b *BudgetChecker) check(ctx context.Context) BudgetStatus {
	usage, err := b.TokenUsageQuery(ctx)
	if err != nil {
		logx.CtxWarnf(ctx, "[cost] query monthly token usage failed: %v", err)
		return BudgetStatus{Budget: b.Cfg.MonthlyBudgetUSD}
	}
	var total float64
	for _, u := range usage {
		// 单价按 model 查（provider 未知时留空，成本估算会按 model-only 匹配）。
		total += b.Calc.Estimate(ctx, "", u.Model, u.InputTokens, u.OutputTokens, u.CacheRead, u.CacheWrite)
	}
	return BudgetStatus{
		Exceeded: total >= b.Cfg.MonthlyBudgetUSD,
		Used:     total,
		Budget:   b.Cfg.MonthlyBudgetUSD,
	}
}
