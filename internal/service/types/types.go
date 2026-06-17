package types

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

// Finding is the canonical code-review finding type shared across all service packages.
type Finding struct {
	ID           string   `json:"id"`
	Severity     string   `json:"severity"`
	Category     string   `json:"category"`
	Path         string   `json:"path"`
	Line         int      `json:"line"`
	EndLine      int      `json:"end_line"`
	Title        string   `json:"title"`
	Body         string   `json:"body"`
	Suggestion   string   `json:"suggestion"`
	Confidence   float64  `json:"confidence"`
	Agents       []string `json:"agents"`
}

// SeverityRank returns a numeric rank for severity comparison.
// Higher value = more severe. Unknown severities return 0.
func SeverityRank(severity string) int {
	switch strings.ToLower(severity) {
	case "critical":
		return 4
	case "major":
		return 3
	case "minor":
		return 2
	case "info":
		return 1
	default:
		return 0
	}
}

// SeverityLabel normalizes a severity string to title case.
func SeverityLabel(severity string) string {
	switch strings.ToLower(severity) {
	case "critical":
		return "Critical"
	case "major":
		return "Major"
	case "minor":
		return "Minor"
	case "info":
		return "Info"
	default:
		return severity
	}
}

// GenerateID creates a unique ID with a timestamp prefix and cryptographic random suffix.
func GenerateID() string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return time.Now().Format("20060102150405") + "-" + hex.EncodeToString(b)
}

// ContainsAny checks if text contains any of the keywords (case-insensitive).
func ContainsAny(text string, keywords ...string) bool {
	lower := strings.ToLower(text)
	for _, kw := range keywords {
		if strings.Contains(lower, strings.ToLower(kw)) {
			return true
		}
	}
	return false
}

// SeverityCount counts findings at or above the given severity threshold.
func SeverityCount(findings []Finding, minSeverity string) int {
	threshold := SeverityRank(minSeverity)
	count := 0
	for _, f := range findings {
		if SeverityRank(f.Severity) >= threshold {
			count++
		}
	}
	return count
}

// ResolvePRNumber extracts the pull request / merge request number from a request.
func ResolvePRNumber(provider string, pullRequestID, mergeRequestIID string) int {
	var num int
	switch provider {
	case "gitlab":
		if mergeRequestIID != "" {
			_, _ = fmt.Sscanf(mergeRequestIID, "%d", &num)
		}
	default:
		if pullRequestID != "" {
			_, _ = fmt.Sscanf(pullRequestID, "%d", &num)
		}
	}
	return num
}

// FormatReviewReport generates a markdown review report. Shared by publisher and writeback.
// If maxComments > 0, findings are truncated. If summaryOnly, findings are omitted.
type FormatFinding struct {
	Severity     string
	Title        string
	Path         string
	Line         int
	Body         string
	ExistingCode string
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
