package workflow

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/yi-nology/agent-harness/budget"
)

// TestComputeDiffBudgetChineseNotUnderestimated 钉住本次重构修的那个 bug。
//
// 此前 diff 预算用扁平「每 4 字符 1 token」换算，而评审内容大量是中文
// （真实密度接近每 1~2 字符 1 token），于是预算**低估**真实用量的 2~4 倍——
// 预算看着够，实际早已越界，表现为模型侧上下文超限而宿主侧看不出异常。
//
// 断言口径：同一份中文文本，预算换算出的字数上限不得让实际 token 数
// 超过 diff 配额（这才是「装得下」的真正含义）。
func TestComputeDiffBudgetChineseNotUnderestimated(t *testing.T) {
	const window = 128000

	b := computeDiffBudget(window)
	if b.MaxChars <= 0 {
		t.Fatalf("MaxChars 应为正数，实际 %d", b.MaxChars)
	}

	// 构造一段中文文本填满上限，估算其 token 数。
	// 中文每 2 字符约 1 token，故 token 数应远大于字符数的一半。
	filler := strings.Repeat("代码评审变更内容", b.MaxChars/4)
	if n := utf8.RuneCountInString(filler); n > b.MaxChars {
		filler = string([]rune(filler)[:b.MaxChars])
	}
	tokens := estimateTokens(filler)

	diffTokenCap := int(float64(b.InputCap) * diffWindowRatio)
	if tokens > diffTokenCap {
		t.Fatalf("中文 diff 在预算上限内估出 %d token，超出 diff 配额 %d —— 预算仍在低估中文密度",
			tokens, diffTokenCap)
	}
}

// TestComputeDiffBudgetReservesOutput 窗口是输入输出共享的，
// 输入侧上限必须先扣掉输出预留——否则模型会生成到一半被截断，
// 用户看到的现象是「回答说到一半没了」，与上下文超限完全无关。
func TestComputeDiffBudgetReservesOutput(t *testing.T) {
	const window = 100000

	b := computeDiffBudget(window)

	outputReserve := int(float64(window) * outputReserveRatio)
	inputCap := budget.MaxInput(window, 0, outputReserve)

	if b.InputCap != inputCap {
		t.Fatalf("输入侧上限应为 %d（已扣输出预留 %d），实际 %d", inputCap, outputReserve, b.InputCap)
	}
	if b.InputCap >= window {
		t.Fatalf("输入侧上限 %d 未扣除输出预留，与窗口 %d 相等", b.InputCap, window)
	}
}

// TestComputeDiffBudgetUnknownWindow 上下文窗口未知时不得声称余量充足。
//
// 底座口径：容量未知时「不知道会不会溢出」与「不会溢出」在行为上完全相反，
// 故 capacity<=0 恒判 critical。
func TestComputeDiffBudgetUnknownWindow(t *testing.T) {
	b := computeDiffBudget(0)
	if b.MaxChars != defaultDiffChars {
		t.Fatalf("窗口未知时应回退到 %d，实际 %d", defaultDiffChars, b.MaxChars)
	}
	if b.InputCap != 0 {
		t.Fatalf("窗口未知时输入侧上限应为 0（未知），实际 %d", b.InputCap)
	}

	// 压力判定：容量未知 → critical。
	_, _, pressure := truncateDiff("任意内容", b)
	if pressure != budget.PressureCritical {
		t.Fatalf("窗口未知时压力档位应为 critical，实际 %s", pressure)
	}
}

// TestComputeDiffBudgetClamps 绝对上下限必须生效，防止极端配置算出荒唐预算。
func TestComputeDiffBudgetClamps(t *testing.T) {
	// 极小窗口：不得低于下限。
	if got := computeDiffBudget(1000).MaxChars; got != minDiffChars {
		t.Fatalf("极小窗口应回退到下限 %d，实际 %d", minDiffChars, got)
	}
	// 极大窗口：不得高于上限。
	if got := computeDiffBudget(100_000_000).MaxChars; got != maxDiffChars {
		t.Fatalf("极大窗口应被上限截断为 %d，实际 %d", maxDiffChars, got)
	}
}

// TestTruncateDiffKeepsMarker 截断必须留痕：模型需要知道内容被截过，
// 否则它会对不完整的 diff 给出「没发现问题」的假结论。
func TestTruncateDiffKeepsMarker(t *testing.T) {
	b := computeDiffBudget(0)
	long := strings.Repeat("x", b.MaxChars+1000)

	got, tokens, _ := truncateDiff(long, b)
	if !strings.Contains(got, "diff truncated") {
		t.Fatalf("截断后应保留截断标记，实际输出尾部为 %q", got[len(got)-40:])
	}
	if tokens <= 0 {
		t.Fatal("截断后的 token 估算应为正数")
	}
}

// TestTruncateDiffShortInputUntouched 未超预算的输入必须原样返回，
// 不得被无意义地改写（改写会破坏 diff 的字节级一致性）。
func TestTruncateDiffShortInputUntouched(t *testing.T) {
	b := computeDiffBudget(0)
	short := "small diff"

	got, _, _ := truncateDiff(short, b)
	if got != short {
		t.Fatalf("未超预算的输入应原样返回，实际 %q", got)
	}
}

// TestTruncateDiffKeepsValidUTF8 钉住「按字节切会把汉字劈成两半」这个 bug。
//
// 旧实现是 diff[:maxChars]——那是**字节**下标。diff 里全是中文时，
// 切点落在多字节字符中间会产出非法 UTF-8，送进模型的是乱码字节序列，
// 模型据此给出的评审意见与真实代码无关。
func TestTruncateDiffKeepsValidUTF8(t *testing.T) {
	b := computeDiffBudget(0)
	// 纯中文，每个 rune 3 字节。刻意让上限不是 3 的倍数，逼出边界切点。
	chinese := strings.Repeat("评审变更", b.MaxChars/2+100)

	got, _, _ := truncateDiff(chinese, b)

	if !utf8.ValidString(got) {
		t.Fatal("截断结果必须是合法 UTF-8 —— 按字节切会劈开多字节字符")
	}
	if strings.ContainsRune(got, utf8.RuneError) {
		t.Fatal("截断结果含替换字符，说明多字节字符被劈开了")
	}
	// rune 数不得超过上限（截断标记另计）。
	body := strings.TrimSuffix(got, "\n... (diff truncated)")
	if n := utf8.RuneCountInString(body); n > b.MaxChars {
		t.Fatalf("截断后 rune 数 %d 超过上限 %d", n, b.MaxChars)
	}
}

// TestTruncateDiffRuneBoundary 截断点必须落在 rune 边界上，
// 不得切出半个字符。
func TestTruncateDiffRuneBoundary(t *testing.T) {
	// 用一个小上限直接测切点函数。
	s := "评审变更内容"
	got := cutAtRuneBoundary(s, 3)

	if !utf8.ValidString(got) {
		t.Fatalf("截断结果非法 UTF-8: %q", got)
	}
	if !strings.HasPrefix(got, "评审变") {
		t.Fatalf("应保留前 3 个 rune，实际 %q", got)
	}
}

// TestResolveContextWindowReadsDynamically 上下文窗口必须**现取**，
// 不能是启动时的快照。
//
// OCR 设置存 DB 且可热更（web console 改完即生效）。若窗口在启动时拍一次快照，
// 管理员调大窗口后 diff 预算仍按旧窗口算——改配置不生效，且没有任何报错。
func TestResolveContextWindowReadsDynamically(t *testing.T) {
	current := 128000
	cfg := WorkflowConfig{
		ContextWindow: func() int { return current },
	}

	if got := cfg.resolveContextWindow(); got != 128000 {
		t.Fatalf("应读到当前窗口 128000，实际 %d", got)
	}

	// 热更后应立刻读到新值。
	current = 64000
	if got := cfg.resolveContextWindow(); got != 64000 {
		t.Fatalf("热更后应读到 64000，实际 %d", got)
	}

	// 未接线时返回 0（预算走保守分支），不得 panic。
	var nilCfg *WorkflowConfig
	if got := nilCfg.resolveContextWindow(); got != 0 {
		t.Fatalf("未接线时应返回 0，实际 %d", got)
	}
	empty := WorkflowConfig{}
	if got := empty.resolveContextWindow(); got != 0 {
		t.Fatalf("未提供窗口函数时应返回 0，实际 %d", got)
	}
}
