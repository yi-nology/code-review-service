package types

import (
	"fmt"
	"strings"
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
		counts["critical"], counts["major"], counts["minor"], counts["info"],
	)

	if summaryOnly || len(findings) == 0 {
		return out.String()
	}

	limit := len(findings)
	if maxComments > 0 && limit > maxComments {
		limit = maxComments
	}
	out.WriteString("\n### Findings\n")
	for i, f := range findings[:limit] {
		fmt.Fprintf(&out, "\n%d. **[%s] %s**", i+1, strings.ToUpper(f.Severity), f.Title)
		if f.Path != "" {
			fmt.Fprintf(&out, " - `%s", f.Path)
			if f.Line > 0 {
				fmt.Fprintf(&out, ":%d", f.Line)
			}
			out.WriteString("`")
		}
		if strings.TrimSpace(f.Body) != "" && strings.TrimSpace(f.Body) != strings.TrimSpace(f.Title) {
			out.WriteString("\n\n   ")
			out.WriteString(indentText(strings.TrimSpace(f.Body), "   "))
		}
		if strings.TrimSpace(f.ExistingCode) != "" {
			out.WriteString("\n\n   **Existing code**\n\n   ```text\n   ")
			out.WriteString(indentText(strings.TrimSpace(f.ExistingCode), "   "))
			out.WriteString("\n   ```")
		}
		if strings.TrimSpace(f.SuggestedCode) != "" {
			out.WriteString("\n\n   **Suggested code**\n\n   ```text\n   ")
			out.WriteString(indentText(strings.TrimSpace(f.SuggestedCode), "   "))
			out.WriteString("\n   ```")
		}
		out.WriteString("\n")
	}
	if limit < len(findings) {
		fmt.Fprintf(&out, "\n_%d additional finding(s) omitted._\n", len(findings)-limit)
	}
	return out.String()
}

func indentText(text, prefix string) string {
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		if i > 0 {
			lines[i] = prefix + line
		}
	}
	return strings.Join(lines, "\n")
}
