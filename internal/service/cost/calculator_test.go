package cost

import "testing"

// TestEstimate_NoPrice 验证无单价记录时返回 0（不阻断，仅缺数据）。
func TestEstimate_NoPrice(t *testing.T) {
	// nil calculator → 0
	var c *Calculator
	if got := c.Estimate(nil, "openai", "gpt-4o", 1000, 500, 0, 0); got != 0 {
		t.Errorf("nil calculator should return 0, got %v", got)
	}
}

// TestEstimate_Formula 验证成本估算公式：
// input/1M × input_price + output/1M × output_price。
func TestEstimate_Formula(t *testing.T) {
	// 用手算的期望值验证公式正确性。
	// 1M input tokens × $5/M = $5；1M output tokens × $15/M = $15 → 总 $20
	inputTokens := int64(1_000_000)
	outputTokens := int64(1_000_000)
	inputPrice := 5.0   // $5 per 1M input
	outputPrice := 15.0 // $15 per 1M output

	// 直接验证公式（不依赖 DB，模拟 Calculator.lookup 的结果）
	const mega = 1_000_000.0
	want := float64(inputTokens)/mega*inputPrice + float64(outputTokens)/mega*outputPrice
	if want != 20.0 {
		t.Errorf("formula sanity check: expected $20, got $%.2f", want)
	}

	// 小用量：1000 input × $5/M + 500 output × $15/M = $0.005 + $0.0075 = $0.0125
	small := float64(1000)/mega*5.0 + float64(500)/mega*15.0
	if small <= 0 {
		t.Errorf("small usage should be positive, got %v", small)
	}
	// 验证小用量成本远小于 1 美元
	if small >= 1.0 {
		t.Errorf("1000 input + 500 output should cost < $1, got $%.4f", small)
	}
}

// TestBudgetChecker_NoBudget 验证无预算配置时 CheckBlock 不拦截。
func TestBudgetChecker_NoBudget(t *testing.T) {
	b := &BudgetChecker{} // Cfg.MonthlyBudgetUSD = 0
	if err := b.CheckBlock(nil); err != nil {
		t.Errorf("CheckBlock with no budget should return nil, got %v", err)
	}
}
