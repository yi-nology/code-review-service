package review

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/oekit/pkg/slicex"
	"github.com/yi-nology/code-review-service/internal/service/review/domain"
)

func BuildAnalysis(result domain.Result) domain.Analysis {
	// PR-10：多 agent 路径下，TechLeadAnalysis 是更高层的总结，优先于 findings 推导。
	if tla := result.TechLeadAnalysis; tla != nil {
		analysis := domain.Analysis{
			Overview:        tla.Summary,
			Risks:           append([]string(nil), tla.Risks...),
			Recommendations: append([]string(nil), tla.Recommendations...),
		}
		// TechLead 没产出 Overview 时回退到 findings 推导
		if analysis.Overview == "" {
			analysis.Overview = fmt.Sprintf("代码评审共审查 %d 个文件，发现 %d 个问题。", result.OCR.FilesReviewed, len(result.Findings))
		}
		return analysis
	}
	if analysis := analysisFromRawOCR(result.RawOCR, result); analysis.Overview != "" {
		return analysis
	}
	return analysisFromFindings(result)
}

func analysisFromFindings(result domain.Result) domain.Analysis {
	if len(result.Findings) == 0 {
		return domain.Analysis{Overview: "代码评审未发现需要报告的代码问题。"}
	}

	analysis := domain.Analysis{
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

func BuildAnalysisFromRaw(raw json.RawMessage, result domain.Result) domain.Analysis {
	result.RawOCR = raw
	return BuildAnalysis(result)
}

func analysisFromRawOCR(raw json.RawMessage, result domain.Result) domain.Analysis {
	if len(raw) == 0 {
		return domain.Analysis{}
	}
	var payload struct {
		ProjectSummary string `json:"project_summary"`
		Comments       []struct {
			Content        string `json:"content"`
			SuggestionCode string `json:"suggestion_code"`
		} `json:"comments"`
	}
	if json.Unmarshal(raw, &payload) != nil {
		return domain.Analysis{}
	}
	analysis := domain.Analysis{
		Overview: fmt.Sprintf(
			"代码评审共审查 %d 个文件，产出 %d 条评审意见。",
			result.OCR.FilesReviewed,
			len(payload.Comments),
		),
	}
	if strings.TrimSpace(payload.ProjectSummary) != "" {
		analysis.Overview = strings.TrimSpace(payload.ProjectSummary)
	}
	if len(payload.Comments) == 0 {
		return analysis
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
			if (strings.Contains(line, "失效") ||
				strings.Contains(line, "影响") ||
				strings.Contains(line, "风险")) && !slicex.Contains(analysis.Risks, line) {
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

func severityLabel(severity string) string {
	switch severity {
	case domain.SeverityCritical:
		return "严重"
	case domain.SeverityMajor:
		return "高风险"
	case domain.SeverityMinor:
		return "中风险"
	default:
		return "提示"
	}
}
