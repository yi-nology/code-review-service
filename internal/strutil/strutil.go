// Package strutil 提供跨服务共用的字符串工具。
//
// 背景：judge/discussion/statuscheck/mcp/eino 曾各自实现 truncate，
// 其中 4 个按字节截断，会在多字节 UTF-8 字符中间切断，产生乱码
// 进入 LLM prompt / GitHub commit status。统一为按 rune 截断的 Truncate。
package strutil

// Truncate 按 rune 截断 s，保证返回值的 rune 数不超过 max；
// 发生截断时以 "…" 结尾（占 1 个 rune）。max <= 0 时返回空串。
//
// 语义对齐 GitHub commit status description 的 140 字符上限场景：
// 截断后的总长度（含省略号）也不会超限。
func Truncate(s string, max int) string {
	if max <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	if max == 1 {
		return "…"
	}
	return string(r[:max-1]) + "…"
}
