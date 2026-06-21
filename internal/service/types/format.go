package types

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
