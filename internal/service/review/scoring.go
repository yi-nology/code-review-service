package review

import (
	"math"
	"strings"

	"github.com/oekit/pkg/mathx"
)

const scoreVersion = "severity-v2"

const (
	OCRStatusSuccess             = "success"
	OCRStatusTimeout             = "timeout"
	OCRStatusSkipped             = "skipped"
	OCRStatusCompletedWithErrors = "completed_with_errors"
)

func isOCRComplete(status string) bool {
	switch status {
	case OCRStatusSuccess, OCRStatusSkipped, OCRStatusCompletedWithErrors:
		return true
	case "":
		return false // 空状态视为未执行 OCR，不算完成
	default:
		return false
	}
}

func BuildReport(result *Result) Report {
	if result == nil {
		return Report{}
	}
	counts := SeverityCounts{}
	for index := range result.Findings {
		result.Findings[index].Category = classifyFinding(result.Findings[index])
		switch result.Findings[index].Severity {
		case SeverityCritical:
			counts.Critical++
		case SeverityMajor:
			counts.Major++
		case SeverityMinor:
			counts.Minor++
		default:
			counts.Info++
		}
	}

	rawDeduction := counts.Critical*25 + counts.Major*12 + counts.Minor*5 + counts.Info*1

	changedLines := result.Scope.ChangedLines
	if changedLines <= 0 {
		changedLines = 100
	}

	volumeMultiplier := mathx.Clamp(100.0/float64(changedLines), 0.2, 3.0)

	deductions := make([]ScoreDeduction, 0, 6)
	totalSeverityDeduction := 0

	if counts.Critical > 0 {
		pts := int(math.Round(float64(counts.Critical*25) * volumeMultiplier))
		deductions = append(deductions, ScoreDeduction{Reason: "严重问题", Points: pts, Count: counts.Critical})
		totalSeverityDeduction += pts
	}
	if counts.Major > 0 {
		pts := int(math.Round(float64(counts.Major*12) * volumeMultiplier))
		deductions = append(deductions, ScoreDeduction{Reason: "高风险问题", Points: pts, Count: counts.Major})
		totalSeverityDeduction += pts
	}
	if counts.Minor > 0 {
		pts := int(math.Round(float64(counts.Minor*5) * volumeMultiplier))
		deductions = append(deductions, ScoreDeduction{Reason: "中风险问题", Points: pts, Count: counts.Minor})
		totalSeverityDeduction += pts
	}
	if counts.Info > 0 {
		pts := int(math.Round(float64(counts.Info*1) * volumeMultiplier))
		deductions = append(deductions, ScoreDeduction{Reason: "提示问题", Points: pts, Count: counts.Info})
		totalSeverityDeduction += pts
	}

	value := 100 - totalSeverityDeduction

	scoringWarnings := 0
	for _, warning := range result.Warnings {
		if warning.Type != "runner_output" {
			scoringWarnings++
		}
	}
	if scoringWarnings > 0 {
		points := mathx.Clamp(scoringWarnings*3, 0, 15)
		value -= points
		deductions = append(deductions, ScoreDeduction{
			Reason: "OCR 审查警告",
			Points: points,
			Count:  scoringWarnings,
		})
	}

	complete := isOCRComplete(result.OCR.Status)
	if !complete {
		value -= 20
		deductions = append(deductions, ScoreDeduction{
			Reason: "审查未完整完成",
			Points: 20,
			Count:  1,
		})
	}

	density := 0.0
	if changedLines > 0 {
		density = float64(rawDeduction) / (float64(changedLines) / 100.0)
	}
	if density < 0.5 && changedLines > 200 && rawDeduction > 0 {
		value += 5
		deductions = append(deductions, ScoreDeduction{
			Reason: "大 PR 低问题密度奖励",
			Points: -5,
			Count:  1,
		})
	}

	value = mathx.Clamp(value, 0, 100)

	return Report{
		OCR:      result.OCR,
		Scope:    result.Scope,
		Warnings: result.Warnings,
		Counts:   counts,
		Complete: complete,
		Generated: result.OCR.Engine != "" ||
			result.OCR.FilesReviewed > 0 ||
			len(result.Findings) > 0,
		Score: Score{
			Value:           value,
			Grade:           scoreGrade(value),
			Version:         scoreVersion,
			Deductions:      deductions,
			ChangedLines:    changedLines,
			FindingsDensity: mathx.Round(density, 2),
		},
	}
}

func CountDiffLines(diff string) int {
	if diff == "" {
		return 0
	}
	count := 0
	for _, line := range strings.Split(diff, "\n") {
		if len(line) == 0 {
			continue
		}
		if line[0] == '+' && !strings.HasPrefix(line, "+++") {
			count++
		}
		if line[0] == '-' && !strings.HasPrefix(line, "---") {
			count++
		}
	}
	return count
}

// CountDiffFiles 统计 diff 中涉及的变更文件数量。
func CountDiffFiles(diff string) int {
	if diff == "" {
		return 0
	}
	count := 0
	for _, line := range strings.Split(diff, "\n") {
		if strings.HasPrefix(line, "diff --git ") {
			count++
		}
	}
	return count
}

func scoreGrade(value int) string {
	switch {
	case value >= 90:
		return "优秀"
	case value >= 80:
		return "良好"
	case value >= 60:
		return "需改进"
	default:
		return "高风险"
	}
}

// ScoreGrade 是 scoreGrade 的 exported 版本，供 workflow 路径同步 FinalReport 评分时使用。
func ScoreGrade(value int) string { return scoreGrade(value) }
