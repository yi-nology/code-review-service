package review

import (
	"github.com/yi-nology/code-review-service/internal/domain/scoring"
	"github.com/yi-nology/code-review-service/internal/service/policy"
	"github.com/yi-nology/code-review-service/internal/service/review/domain"
)

// 评分逻辑已提取到 scoring 包（高内聚），此处保留常量/函数为向后兼容别名。

const (
	OCRStatusSuccess             = scoring.OCRStatusSuccess
	OCRStatusTimeout             = scoring.OCRStatusTimeout
	OCRStatusSkipped             = scoring.OCRStatusSkipped
	OCRStatusCompletedWithErrors = scoring.OCRStatusCompletedWithErrors
)

func isOCRComplete(status string) bool { return scoring.IsOCRComplete(status) }

// BuildReport 基于审查结果组装最终报告，评分委托给 scoring.Compute。
func BuildReport(result *domain.Result) domain.Report {
	if result == nil {
		return domain.Report{}
	}
	counts := domain.SeverityCounts{}
	for index := range result.Findings {
		result.Findings[index].Category = classifyFinding(result.Findings[index])
		switch result.Findings[index].Severity {
		case domain.SeverityCritical:
			counts.Critical++
		case domain.SeverityMajor:
			counts.Major++
		case domain.SeverityMinor:
			counts.Minor++
		default:
			counts.Info++
		}
	}

	warningCount := 0
	for _, warning := range result.Warnings {
		if warning.Type != "runner_output" {
			warningCount++
		}
	}

	complete := isOCRComplete(result.OCR.Status)

	return domain.Report{
		OCR:      result.OCR,
		Scope:    result.Scope,
		Warnings: result.Warnings,
		Counts:   counts,
		Complete: complete,
		Generated: result.OCR.Engine != "" ||
			result.OCR.FilesReviewed > 0 ||
			len(result.Findings) > 0,
		Score: scoring.Compute(counts, result.Scope.ChangedLines, warningCount, complete),
	}
}

// CountDiffLines / CountDiffFiles 向后兼容别名，供 executor 等已 import review 的包使用。
func CountDiffLines(diff string) int { return scoring.CountDiffLines(diff) }
func CountDiffFiles(diff string) int { return scoring.CountDiffFiles(diff) }

// PolicyFindings 把 domain findings 转为门禁引擎输入。
// scan 路径与 eino 执行器共用（此前两处各持一份相同的转换）。
func PolicyFindings(findings []domain.Finding) []policy.Finding {
	out := make([]policy.Finding, 0, len(findings))
	for _, finding := range findings {
		out = append(out, policy.Finding{Severity: finding.Severity, Path: finding.Path})
	}
	return out
}
