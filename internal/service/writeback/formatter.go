package writeback

import (
	"fmt"

	"github.com/yi-nology/code-review-service/internal/service/review"
)

// FormatCollapsibleSection formats a collapsible section
func FormatCollapsibleSection(title string, content string) string {
	return fmt.Sprintf("<details>\n<summary>%s</summary>\n\n%s\n\n</details>", title, content)
}

// FormatCodeBlock formats a code block with language
func FormatCodeBlock(code string, language string) string {
	return fmt.Sprintf("```%s\n%s\n```", language, code)
}

// FormatSuggestion formats a code suggestion
func FormatSuggestion(code string, path string, line int) string {
	return fmt.Sprintf("```suggestion\n%s\n```\n\n📍 `%s:%d`", code, path, line)
}

// TranslateToHumanReadable translates technical findings to human-readable format
func TranslateToHumanReadable(findings []review.Finding) []TranslatedFinding {
	var translated []TranslatedFinding

	for _, f := range findings {
		tf := TranslatedFinding{
			Original: f,
			Title:    translateTitle(f),
			Summary:  translateSummary(f),
			Impact:   translateImpact(f),
			Action:   translateAction(f),
		}
		translated = append(translated, tf)
	}

	return translated
}

// TranslatedFinding represents a human-readable finding
type TranslatedFinding struct {
	Original review.Finding
	Title    string
	Summary  string
	Impact   string
	Action   string
}

// translateTitle translates the finding title
func translateTitle(f review.Finding) string {
	// Keep original title but add severity context
	switch f.Severity {
	case review.SeverityCritical:
		return fmt.Sprintf("Critical Issue: %s", f.Title)
	case review.SeverityMajor:
		return fmt.Sprintf("Important Issue: %s", f.Title)
	default:
		return f.Title
	}
}

// translateSummary translates the finding summary
func translateSummary(f review.Finding) string {
	if f.Body != "" {
		return f.Body
	}

	// Generate a summary based on category
	switch f.Category {
	case "security":
		return "This finding identifies a potential security vulnerability that should be addressed."
	case "performance":
		return "This finding identifies a potential performance issue that could affect application responsiveness."
	case "bug":
		return "This finding identifies a potential bug that could cause incorrect behavior."
	case "style":
		return "This finding identifies a style issue that could affect code readability."
	default:
		return "This finding identifies an area for improvement in the code."
	}
}

// translateImpact translates the potential impact
func translateImpact(f review.Finding) string {
	switch f.Severity {
	case review.SeverityCritical:
		return "High impact - Could cause system failures, security breaches, or data loss."
	case review.SeverityMajor:
		return "Medium impact - Could cause bugs, performance issues, or maintenance difficulties."
	case review.SeverityMinor:
		return "Low impact - Could improve code quality and maintainability."
	default:
		return "Minimal impact - Suggestion for improvement."
	}
}

// translateAction translates the recommended action
func translateAction(f review.Finding) string {
	if f.SuggestionCode != "" {
		return f.SuggestionCode
	}

	switch f.Severity {
	case review.SeverityCritical:
		return "Immediate action required. Please fix this issue before merging."
	case review.SeverityMajor:
		return "Strongly recommended to fix. Consider addressing this issue."
	case review.SeverityMinor:
		return "Consider fixing if time permits. This is a suggestion for improvement."
	default:
		return "Optional improvement. Consider if it aligns with your coding standards."
	}
}
