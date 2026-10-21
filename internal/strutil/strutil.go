// Package strutil 提供跨服务共用的字符串工具。
//
// 背景：judge/discussion/statuscheck/mcp/eino 曾各自实现 truncate，
// 其中 4 个按字节截断，会在多字节 UTF-8 字符中间切断，产生乱码
// 进入 LLM prompt / GitHub commit status。统一为按 rune 截断的 Truncate。
//
// rune 安全的截断本身委托 oekit/pkg/stringx（TruncRunes），本包只负责
// **本服务特有的那层语义**：截断后连省略号算在内，总长也不超过上限。
package strutil

import "github.com/oekit/pkg/stringx"

// Truncate 按 rune 截断 s，保证返回值的 rune 数不超过 max；
// 发生截断时以 "…" 结尾（占 1 个 rune）。max <= 0 时返回空串。
//
// 语义对齐 GitHub commit status description 的 140 字符上限场景：
// 截断后的总长度（含省略号）也不会超限。
//
// 与 oekit/pkg/stringx.TruncEllipsis 的差别：后者是「截到 n 位再补省略号」，
// 总长 n+1；本函数要的是「总长 ≤ max」，故留给省略号一位。
func Truncate(s string, max int) string {
	if max <= 0 {
		return ""
	}
	if _, truncated := stringx.TruncRunes(s, max); !truncated {
		return s
	}
	// 发生截断：留 max-1 位给内容，末位是省略号，总长 = max。
	out, _ := stringx.TruncRunes(s, max-1)
	return out + "…"
}
