package writeback

import (
	"fmt"
	"strings"

	"github.com/oekit/pkg/stringx"
	types "github.com/yi-nology/code-review-service/internal/domain/finding"
	"github.com/yi-nology/code-review-service/internal/domain/review"
)

type ValidatedComment struct {
	Comment
	ValidLocation bool
}

func ValidateFindings(findings []domain.Finding, diffIndex *DiffIndex) []ValidatedComment {
	var result []ValidatedComment
	for _, f := range findings {
		comment := findingToComment(f)
		if comment.Path == "" {
			result = append(result, ValidatedComment{Comment: comment, ValidLocation: false})
			continue
		}

		path := normalizePath(f.Path)
		if path == "" {
			result = append(result, ValidatedComment{Comment: comment, ValidLocation: false})
			continue
		}
		comment.Path = path

		if diffIndex == nil {
			result = append(result, ValidatedComment{Comment: comment, ValidLocation: comment.Range.Start > 0})
			continue
		}

		valid := false
		if comment.Range.Start > 0 && comment.Range.End >= comment.Range.Start {
			valid = diffIndex.ContainsRange(path, comment.Range)
		} else if comment.Range.Start > 0 {
			valid = diffIndex.Contains(path, comment.Range.Side, comment.Range.Start)
		}

		result = append(result, ValidatedComment{Comment: comment, ValidLocation: valid})
	}
	return result
}

func findingToComment(f domain.Finding) Comment {
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

func FormatInlineComment(severity string, title, body, suggestionCode, existingCode string, startLine int, key string) string {
	sev := severity
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

func FormatSummaryBody(summary string, findings []domain.Finding, maxComments int) string {
	ff := make([]types.FormatFinding, len(findings))
	for i, f := range findings {
		ff[i] = types.FormatFinding{
			Severity:      f.Severity,
			Title:         f.Title,
			Path:          f.Path,
			Line:          f.Line,
			Body:          f.Body,
			ExistingCode:  f.ExistingCode,
			SuggestedCode: f.SuggestionCode,
		}
	}
	return types.FormatReviewReport("<!-- review-service:summary -->", summary, ff, maxComments, false)
}

// FormatStaleBanner 生成过期降级回写摘要顶部的提示横幅。
func FormatStaleBanner(expected, actual string) string {
	return fmt.Sprintf(
		"> ⚠️ **本评审已过期（已降级为仅发布摘要）**\n"+
			"> 评审基于提交 `%s`，但合并请求已更新到 `%s`。\n"+
			"> 行内评论因代码行号变化无法定位，本次仅发布评审摘要。**请对最新提交重新触发评审**以获取行内评论。",
		ShortSHA(expected), ShortSHA(actual),
	)
}

// ShortSHA 截取 commit SHA 的前 8 位用于展示。
func ShortSHA(sha string) string {
	return stringx.FirstN(sha, 8)
}
