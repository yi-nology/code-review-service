package runner

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/yi-nology/code-review-service/internal/service/review/domain"
	"github.com/oekit/pkg/stringx"
)

func ParseOpenCodeReviewOutput(output []byte, defaultSeverity string) (domain.Result, error) {
	output = bytes.TrimSpace(output)
	if len(output) == 0 {
		return domain.Result{}, fmt.Errorf("open-code-review produced empty output")
	}
	jsonStart := bytes.IndexByte(output, '{')
	if jsonStart < 0 {
		return domain.Result{}, fmt.Errorf("open-code-review output does not contain JSON")
	}
	prefix := strings.TrimSpace(string(output[:jsonStart]))
	payload := bytes.TrimSpace(output[jsonStart:])

	var parsed openCodeReviewOutput
	if err := json.Unmarshal(payload, &parsed); err != nil {
		return domain.Result{}, fmt.Errorf("parse open-code-review output: %w", err)
	}

	findings := make([]domain.Finding, 0, len(parsed.Comments))
	for _, comment := range parsed.Comments {
		line := comment.StartLine
		if line == 0 {
			line = comment.EndLine
		}
		findings = append(findings, domain.Finding{
			Severity:       domain.InferSeverity(comment.Content, defaultSeverity),
			Path:           comment.Path,
			Line:           line,
			StartLine:      comment.StartLine,
			EndLine:        comment.EndLine,
			Title:          firstLine(comment.Content),
			Body:           strings.TrimSpace(comment.Content),
			ExistingCode:   strings.TrimSpace(comment.ExistingCode),
			SuggestionCode: strings.TrimSpace(comment.SuggestionCode),
			Thinking:       strings.TrimSpace(comment.Thinking),
		})
	}

	cleaned := sanitizeRawOCR(payload)
	result := domain.Result{
		Summary: summaryText(parsed, len(findings)),
		OCR: domain.OCRMetadata{
			Engine: "open-code-review",
			Status: parsed.Status,
		},
		Warnings: parseWarnings(parsed.Warnings),
		Findings: findings,
		RawOCR:   append(json.RawMessage(nil), cleaned...),
	}
	if prefix != "" {
		result.Warnings = append([]domain.OCRWarning{{
			Type:    "runner_output",
			Message: prefix,
		}}, result.Warnings...)
	}
	if parsed.Summary != nil {
		result.OCR.FilesReviewed = parsed.Summary.FilesReviewed
		result.OCR.Comments = parsed.Summary.Comments
		result.OCR.TotalTokens = parsed.Summary.TotalTokens
		result.OCR.InputTokens = parsed.Summary.InputTokens
		result.OCR.OutputTokens = parsed.Summary.OutputTokens
		result.OCR.CacheReadTokens = parsed.Summary.CacheReadTokens
		result.OCR.CacheWriteTokens = parsed.Summary.CacheWriteTokens
		result.OCR.Elapsed = parsed.Summary.Elapsed
	}
	if parsed.ToolCalls != nil {
		result.OCR.ToolCalls = parsed.ToolCalls
	}
	if strings.TrimSpace(parsed.ProjectSummary) != "" {
		result.OCR.ProjectSummary = strings.TrimSpace(parsed.ProjectSummary)
	}
	return result, nil
}

type openCodeReviewOutput struct {
	Status         string                    `json:"status"`
	Message        string                    `json:"message"`
	Summary        *openCodeReviewSummary    `json:"summary"`
	ToolCalls      *domain.ToolCallsMetadata `json:"tool_calls"`
	Comments       []openCodeReviewComment   `json:"comments"`
	Warnings       []json.RawMessage         `json:"warnings"`
	ProjectSummary string                    `json:"project_summary"`
}

type openCodeReviewSummary struct {
	FilesReviewed    int64  `json:"files_reviewed"`
	Comments         int64  `json:"comments"`
	TotalTokens      int64  `json:"total_tokens"`
	InputTokens      int64  `json:"input_tokens"`
	OutputTokens     int64  `json:"output_tokens"`
	CacheReadTokens  int64  `json:"cache_read_tokens"`
	CacheWriteTokens int64  `json:"cache_write_tokens"`
	Elapsed          string `json:"elapsed"`
}

type openCodeReviewComment struct {
	Path           string `json:"path"`
	Content        string `json:"content"`
	SuggestionCode string `json:"suggestion_code"`
	ExistingCode   string `json:"existing_code"`
	StartLine      int    `json:"start_line"`
	EndLine        int    `json:"end_line"`
	Thinking       string `json:"thinking"`
}

func firstLine(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "代码评审意见"
	}
	line, _, _ := strings.Cut(value, "\n")
	return stringx.FirstN(line, 120)
}

func summaryText(parsed openCodeReviewOutput, findings int) string {
	if parsed.Summary != nil {
		return fmt.Sprintf(
			"代码评审完成：发现 %d 个问题，审查 %d 个文件，使用 %d Tokens",
			findings,
			parsed.Summary.FilesReviewed,
			parsed.Summary.TotalTokens,
		)
	}
	if strings.TrimSpace(parsed.Message) != "" {
		return parsed.Message
	}
	return fmt.Sprintf("代码评审完成：发现 %d 个问题", findings)
}

func parseWarnings(rawWarnings []json.RawMessage) []domain.OCRWarning {
	warnings := make([]domain.OCRWarning, 0, len(rawWarnings))
	for _, raw := range rawWarnings {
		var fields map[string]any
		if err := json.Unmarshal(raw, &fields); err != nil {
			warnings = append(warnings, domain.OCRWarning{Message: string(raw)})
			continue
		}
		warning := domain.OCRWarning{
			Type:    firstString(fields, "type", "code", "kind"),
			Message: firstString(fields, "message", "error", "reason"),
			Path:    firstString(fields, "path", "file"),
		}
		if warning.Message == "" {
			compact, _ := json.Marshal(fields)
			warning.Message = string(compact)
		}
		warnings = append(warnings, warning)
	}
	return warnings
}

func firstString(fields map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := fields[key].(string); ok && strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func sanitizeRawOCR(payload json.RawMessage) json.RawMessage {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(payload, &raw); err != nil {
		return payload
	}

	commentsRaw, ok := raw["comments"]
	if !ok {
		return payload
	}

	var comments []map[string]json.RawMessage
	if err := json.Unmarshal(commentsRaw, &comments); err != nil {
		return payload
	}

	changed := false
	for i, c := range comments {
		if _, has := c["thinking"]; has {
			delete(c, "thinking")
			comments[i] = c
			changed = true
		}
	}

	if !changed {
		return payload
	}

	cleanedComments, err := json.Marshal(comments)
	if err != nil {
		return payload
	}
	raw["comments"] = cleanedComments

	cleaned, err := json.Marshal(raw)
	if err != nil {
		return payload
	}
	return cleaned
}
