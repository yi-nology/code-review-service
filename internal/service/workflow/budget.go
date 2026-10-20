package workflow

import (
	"context"
	"strings"
	"unicode/utf8"

	"github.com/yi-nology/agent-harness/budget"
	"github.com/yi-nology/agent-memory/ports"
)

// budget.go：评审输入侧的**上下文预算**。
//
// 此前 diff 截断用一条手写公式：`ContextWindow * 0.35 / 4` 字符。
// 那个 /4 是「英文平均 4 字符 1 token」的经验值，而本服务的评审内容
// **大量是中文**（diff、注释、commit message、文档）。中文的真实密度接近
// 每 1~2 字符 1 token，沿用 /4 会把估算值低估到真实用量的 1/2~1/4——
// 预算看着够，实际早已越界，表现为模型侧「上下文超限」或输出被截断，
// 而宿主侧看不出任何异常。
//
// 现在这段估算与判定都交给底座：
//   - token 估算：agent-memory/ports.StaticEstimator
//     （CJK 每 2 字符 1 token、其余每 4 字符 1 token，逐段向上取整）
//   - 窗口扣减与压力分档：agent-harness/budget
//     （MaxInput 扣输出预留、Classify 判 normal/tight/critical）
//
// 口径单源：估算器与分档表各只有一处，宿主不再另立一套。
//
// 注意：底座的 budget 是**机制面**（怎么计量、怎么判压力），
// 「diff 占窗口多大比例」是**产品决策**，归宿主——所以比例系数留在这里。

const (
	// diffWindowRatio 是 diff 允许占用的**输入侧**上下文比例。
	//
	// 留剩下的部分给系统提示、规则文件、召回记忆与模型输出。
	// 取 0.35 是沿用重构前的经验值，本次重构不改变产品口径——
	// 改变的是**怎么换算**（token 感知而非字符感知），不是占比本身。
	diffWindowRatio = 0.35

	// minDiffChars / maxDiffChars 是 diff 字数（rune）的绝对上下限。
	//
	// 下限保证小窗口配置下 diff 不至于短到没有评审价值；
	// 上限防止超长 diff 把预算算爆，也给内存占用封顶。
	minDiffChars = 10000
	maxDiffChars = 500000

	// defaultDiffChars 是**未配置上下文窗口**时的兜底上限。
	//
	// 此时无法按窗口推算（底座 Classify 会判 critical：容量未知即不可声称余量充足），
	// 只能给一个固定上限。沿用重构前的 30000 口径。
	defaultDiffChars = 30000

	// outputReserveRatio 是为模型输出预留的窗口比例。
	//
	// 窗口是输入输出共享的：输入占满时，模型会**生成到一半被截断**，
	// 用户看到的现象是「回答说到一半没了」，与上下文超限完全无关，
	// 排查方向会被带偏。预留先扣，截断就不会发生在生成为零的时候。
	outputReserveRatio = 0.25

	// cjkCharsPerToken 是中文的保守字符/token 比例。
	//
	// 与底座 StaticEstimator 的 CJK 系数一致（每 2 字符 1 token）。
	// 保守取法的意义：按最坏密度（纯中文）换算出的字符上限，
	// 在英文内容上只会**用不满**预算（安全），在中文内容上刚好装得下。
	// 若换成英文的 4 字符/token，中文 diff 会再次超限——那正是本次要修的 bug。
	cjkCharsPerToken = 2
)

// tokenEstimator 是本包用的 token 估算器。
//
// 用底座 memory 的 StaticEstimator：它按 CJK/ASCII 分段折算，
// 逐段向上取整（合总再取整会累积成规模的低估，而低估的方向恰好是静默的）。
var tokenEstimator = ports.StaticEstimator{}

// diffBudget 是一次评审的 diff 输入预算。
type diffBudget struct {
	// MaxChars 是 diff 截断后的字数上限（rune 数，不是字节数）。
	MaxChars int
	// InputCap 是输入侧可用 token 上限（已扣输出预留）。
	// 0 表示上下文窗口未知，无法推导。
	InputCap int
	// Window 是推导预算所依据的上下文窗口（0 = 未知）。
	Window int
}

// computeDiffBudget 依据上下文窗口推导 diff 预算。
//
// window <= 0 表示未配置窗口：此时只能给固定上限。
func computeDiffBudget(window int) diffBudget {
	if window <= 0 {
		return diffBudget{
			MaxChars: defaultDiffChars,
			InputCap: 0, // 窗口未知 → 输入侧上限未知
			Window:   0,
		}
	}

	// 先扣输出预留，得到输入侧可用上限。
	outputReserve := int(float64(window) * outputReserveRatio)
	inputCap := budget.MaxInput(window, 0, outputReserve)

	// 再按产品口径取 diff 占输入侧的比例。
	diffTokens := int(float64(inputCap) * diffWindowRatio)
	if diffTokens < 1 {
		diffTokens = 1
	}

	// token → 字数：用保守比例换算（每 token 2 字符），
	// 保证换算后的字数在中文内容上不会超出 token 上限。
	maxChars := diffTokens * cjkCharsPerToken
	if maxChars < minDiffChars {
		maxChars = minDiffChars
	}
	if maxChars > maxDiffChars {
		maxChars = maxDiffChars
	}

	return diffBudget{
		MaxChars: maxChars,
		InputCap: inputCap,
		Window:   window,
	}
}

// estimateTokens 估算一段文本的 token 数。
//
// 估算器不返回 error（静态代理），但接口签名带 error——这里显式吞掉它：
// 估算失败时的正确退路是「按 0 算」而不是「按未知算」，
// 因为未知会让调用方不敢做本该做的截断。
func estimateTokens(text string) int {
	// 静态代理不读 ctx（上游实现签名为 `_ context.Context`），
	// 但仍传真实 ctx：将来换成会读 ctx 的估算器时，这里不需要跟着改。
	n, err := tokenEstimator.Estimate(context.Background(), text)
	if err != nil || n < 0 {
		return 0
	}
	return n
}

// truncateDiff 按预算截断 diff，并回报截断后的真实 token 估算与压力档位。
//
// 截断标记刻意保留在正文里：模型需要知道内容被截过，
// 否则它会对不完整的 diff 给出「没发现问题」的假结论。
//
// 截断点按 **rune 边界**取，不按字节切：diff 里全是中文时，
// 按字节切会把一个汉字劈成两半，产出非法 UTF-8 送进模型。
// 那个乱码不是「显示难看」，而是会让模型读到无意义的字节序列，
// 进而给出与真实代码无关的评审意见。
func truncateDiff(diff string, b diffBudget) (string, int, budget.Pressure) {
	if b.MaxChars <= 0 || utf8.RuneCountInString(diff) <= b.MaxChars {
		tokens := estimateTokens(diff)
		return diff, tokens, budget.Classify(tokens, b.InputCap)
	}

	truncated := cutAtRuneBoundary(diff, b.MaxChars)
	tokens := estimateTokens(truncated)
	return truncated, tokens, budget.Classify(tokens, b.InputCap)
}

// cutAtRuneBoundary 在不超过 limit 个 rune 的前提下截断 s，
// 并保留截断标记。
//
// 切点回退到最近的合法 rune 边界：若第 limit 个 rune 之后有半个多字节字符，
// 就把它整块去掉。回退量最多 3 字节（UTF-8 最长编码 4 字节），
// 对上限的影响可以忽略，但保证输出恒为合法 UTF-8。
func cutAtRuneBoundary(s string, limit int) string {
	count := 0
	for i := range s {
		if count == limit {
			// s[i] 是第 limit+1 个 rune 的起始字节。
			// 它前面的字节都已计入 count，截断点取 i 即可保持 rune 边界。
			return s[:i] + "\n... (diff truncated)"
		}
		count++
	}
	return s + "\n... (diff truncated)"
}

// diffPressure 是给日志/审计用的压力描述（档位名）。
func diffPressure(p budget.Pressure) string {
	return strings.ToLower(p.String())
}
