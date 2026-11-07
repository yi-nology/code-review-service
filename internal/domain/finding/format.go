package finding

import (
	"fmt"
	"strings"

	"github.com/oekit/pkg/stringx"
)

type FormatFinding struct {
	Severity      string
	Title         string
	Path          string
	Line          int
	Body          string
	ExistingCode  string
	SuggestedCode string
}

func FormatReviewReport(reportMarker string, summary string, findings []FormatFinding, maxComments int, summaryOnly bool) string {
	var out strings.Builder
	out.WriteString(reportMarker)
	out.WriteString("\n## AI Code Review\n\n")
	out.WriteString(summary)
	out.WriteString("\n\n")

	counts := map[string]int{}
	for _, f := range findings {
		counts[f.Severity]++
	}
	fmt.Fprintf(&out,
		"**Findings:** %d critical, %d major, %d minor, %d info\n",
		counts["critical"], counts["major"], counts["minor"],
		counts["info"])

	// line=0 findings 的下游产品决策（issue #2）：统计分母显式带出——
	// 行内评论/patch 只覆盖有有效行号的部分，读者需要知道这个差值存在，
	// 否则「没审到」与「没法锚定」不可分辨。
	locatable := 0
	for _, f := range findings {
		if f.Line > 0 {
			locatable++
		}
	}
	if unlocatable := len(findings) - locatable; unlocatable > 0 {
		fmt.Fprintf(&out,
			"**Unlocatable findings:** %d（无有效行号，未包含在行内评论与 patch 候选中）\n",
			unlocatable)
	}

	if summaryOnly {
		return out.String()
	}

	shown := 0
	for _, f := range findings {
		if maxComments > 0 && shown >= maxComments {
			remaining := len(findings) - shown
			fmt.Fprintf(&out, "\n*%d additional finding(s) omitted.*\n", remaining)
			break
		}
		severity := strings.ToUpper(f.Severity)
		fmt.Fprintf(&out, "\n### [%s] %s\n", severity, f.Title)
		fmt.Fprintf(&out, "**File:** `%s`", f.Path)
		if f.Line > 0 {
			fmt.Fprintf(&out, " (line %d)", f.Line)
		}
		out.WriteString("\n\n")
		if f.Body != "" {
			out.WriteString(stringx.IndentText(f.Body, "> "))
			out.WriteString("\n\n")
		}
		shown++
	}

	return out.String()
}
