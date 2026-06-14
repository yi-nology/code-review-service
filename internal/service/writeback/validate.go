package writeback

import (
	"fmt"
	"strings"

	"github.com/yi-nology/code-review-service/internal/service/review"

	"github.com/cloudwego/hertz/pkg/common/hlog"
)

const reportMarker = "<!-- review-service:summary -->"

type ValidatedComment struct {
	Comment
	ValidLocation bool
}

func ValidateFindings(findings []review.Finding, diffIndex *DiffIndex) []ValidatedComment {
	var result []ValidatedComment
	for _, f := range findings {
		comment := findingToComment(f)
		if comment.Path == "" || diffIndex == nil {
			result = append(result, ValidatedComment{Comment: comment, ValidLocation: false})
			continue
		}

		path := normalizePath(f.Path)
		if path == "" {
			result = append(result, ValidatedComment{Comment: comment, ValidLocation: false})
			continue
		}
		comment.Path = path

		valid := false
		if comment.Range.Start > 0 && comment.Range.End >= comment.Range.Start {
			valid = diffIndex.ContainsRange(path, comment.Range)
		} else if comment.Range.Start > 0 {
			valid = diffIndex.Contains(path, comment.Range.Side, comment.Range.Start)
		}

		hlog.Info("validate finding",
			"path", path,
			"start", comment.Range.Start,
			"end", comment.Range.End,
			"valid", valid,
		)
		result = append(result, ValidatedComment{Comment: comment, ValidLocation: valid})
	}
	return result
}

func findingToComment(f review.Finding) Comment {
	startLine := f.StartLine
	if startLine == 0 {
		startLine = f.Line
	}
	endLine := f.EndLine
	if endLine == 0 {
		endLine = startLine
	}

	side := SideNew
	body := f.Body
	if body == "" {
		body = f.Title
	}

	key := FindingKey(f.Category, f.Path, side, startLine, body)

	return Comment{
		Key:        key,
		Path:       f.Path,
		Body:       body,
		Range:      LineRange{Start: startLine, End: endLine, Side: side},
		Suggestion: f.SuggestionCode,
	}
}

func FormatInlineComment(severity review.Severity, title, body, suggestionCode, existingCode string, startLine int, key string) string {
	sev := string(severity)
	if sev == "" {
		sev = "MAJOR"
	}

	result := fmt.Sprintf("**[%s] %s**\n\n%s", strings.ToUpper(sev), title, body)

	if suggestionCode != "" && existingCode != "" && startLine > 0 {
		result += fmt.Sprintf("\n\n```suggestion\n%s\n```", suggestionCode)
	}

	result += fmt.Sprintf("\n\n<!-- review-service:finding:%s -->", key)
	return result
}

func FormatSummaryBody(summary string, findings []review.Finding, maxComments int) string {
	var out strings.Builder
	out.WriteString(reportMarker)
	out.WriteString("\n## AI Code Review\n\n")
	out.WriteString(summary)
	out.WriteString("\n\n")

	counts := make(map[review.Severity]int)
	for _, f := range findings {
		counts[f.Severity]++
	}
	fmt.Fprintf(&out,
		"**Findings:** %d critical, %d major, %d minor, %d info\n",
		counts[review.SeverityCritical],
		counts[review.SeverityMajor],
		counts[review.SeverityMinor],
		counts[review.SeverityInfo],
	)

	if len(findings) == 0 {
		return out.String()
	}

	limit := len(findings)
	if maxComments > 0 && limit > maxComments {
		limit = maxComments
	}
	out.WriteString("\n### Findings\n")
	for i, f := range findings[:limit] {
		fmt.Fprintf(&out, "\n%d. **[%s] %s**", i+1, strings.ToUpper(string(f.Severity)), f.Title)
		if f.Path != "" {
			fmt.Fprintf(&out, " - `%s", f.Path)
			if f.Line > 0 {
				fmt.Fprintf(&out, ":%d", f.Line)
			}
			out.WriteString("`")
		}
		out.WriteString("\n")
	}
	if limit < len(findings) {
		fmt.Fprintf(&out, "\n_%d additional finding(s) omitted._\n", len(findings)-limit)
	}
	return out.String()
}
