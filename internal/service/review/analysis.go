package review

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/oekit/pkg/slicex"
)

func BuildAnalysis(result Result) Analysis {
	if analysis := analysisFromRawOCR(result.RawOCR, result); analysis.Overview != "" {
		return analysis
	}
	return analysisFromFindings(result)
}

func analysisFromFindings(result Result) Analysis {
	if len(result.Findings) == 0 {
		return Analysis{Overview: "代码评审未发现需要报告的代码问题。"}
	}

	analysis := Analysis{
		Overview: fmt.Sprintf(
			"代码评审共审查 %d 个文件，发现 %d 个问题。",
			result.OCR.FilesReviewed,
			len(result.Findings),
		),
	}
	for _, finding := range result.Findings {
		location := finding.Path
		if finding.StartLine > 0 {
			location = fmt.Sprintf("%s:%d", location, finding.StartLine)
		}
		analysis.RootCause = append(analysis.RootCause, strings.TrimSpace(finding.Body))
		analysis.Risks = append(analysis.Risks, fmt.Sprintf(
			"%s（%s，%s）",
			finding.Title,
			severityLabel(finding.Severity),
			location,
		))
		for _, line := range strings.Split(finding.Body, "\n") {
			line = strings.TrimSpace(line)
			if isRecommendationLine(line) && !slicex.Contains(analysis.Recommendations, line) {
				analysis.Recommendations = append(analysis.Recommendations, line)
			}
		}
		if strings.TrimSpace(finding.SuggestionCode) != "" && !slicex.Contains(analysis.Recommendations, "按问题报告中的\u201c建议改进后\u201d代码调整实现。") {
			analysis.Recommendations = append(analysis.Recommendations, "按问题报告中的\u201c建议改进后\u201d代码调整实现。")
		}
	}
	return analysis
}

func BuildAnalysisFromRaw(raw json.RawMessage, result Result) Analysis {
	result.RawOCR = raw
	return BuildAnalysis(result)
}

func analysisFromRawOCR(raw json.RawMessage, result Result) Analysis {
	if len(raw) == 0 {
		return Analysis{}
	}
	var payload struct {
		Comments []struct {
			Content        string `json:"content"`
			SuggestionCode string `json:"suggestion_code"`
		} `json:"comments"`
	}
	if json.Unmarshal(raw, &payload) != nil || len(payload.Comments) == 0 {
		return Analysis{}
	}
	analysis := Analysis{
		Overview: fmt.Sprintf(
			"代码评审共审查 %d 个文件，产出 %d 条评审意见。",
			result.OCR.FilesReviewed,
			len(payload.Comments),
		),
	}
	for _, comment := range payload.Comments {
		content := strings.TrimSpace(comment.Content)
		if content == "" {
			continue
		}
		analysis.RootCause = append(analysis.RootCause, content)
		inRecommendations := false
		for _, line := range strings.Split(content, "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "建议") {
				inRecommendations = true
				continue
			}
			if strings.Contains(line, "失效") ||
				strings.Contains(line, "影响") ||
				strings.Contains(line, "风险") && !slicex.Contains(analysis.Risks, line) {
				analysis.Risks = append(analysis.Risks, line)
			}
			if inRecommendations && line != "" && !slicex.Contains(analysis.Recommendations, line) {
				analysis.Recommendations = append(analysis.Recommendations, line)
			}
		}
		suggestionText := "模型已提供可直接参考的建议代码，见问题报告中的\u201c建议改进后\u201d。"
		if strings.TrimSpace(comment.SuggestionCode) != "" && !slicex.Contains(analysis.Recommendations, suggestionText) {
			analysis.Recommendations = append(analysis.Recommendations, suggestionText)
		}
	}
	return analysis
}

func isRecommendationLine(line string) bool {
	if line == "" {
		return false
	}
	lower := strings.ToLower(line)
	return strings.Contains(lower, "建议") ||
		strings.HasPrefix(line, "1.") ||
		strings.HasPrefix(line, "2.") ||
		strings.HasPrefix(line, "3.") ||
		strings.HasPrefix(line, "4.")
}

func severityLabel(severity Severity) string {
	switch severity {
	case SeverityCritical:
		return "严重"
	case SeverityMajor:
		return "高风险"
	case SeverityMinor:
		return "中风险"
	default:
		return "提示"
	}
}
