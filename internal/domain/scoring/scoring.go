// Package scoring 是纯粹的评分计算模块，不依赖任何 service 层。
//
// 它是评分逻辑的单一数据源（single source of truth）：runner 路径（review.BuildReport）
// 和 workflow 路径（workflow.generateFinalReport）都调用 Compute，确保两条路径的分数
// 与扣分明细完全一致。
package scoring

import (
	"math"
	"strings"

	"github.com/oekit/pkg/mathx"
)

const Version = "severity-v2"

const (
	OCRStatusSuccess             = "success"
	OCRStatusTimeout             = "timeout"
	OCRStatusSkipped             = "skipped"
	OCRStatusCompletedWithErrors = "completed_with_errors"
)

// 严重级别权重（每个问题扣几分，rawDeduction 用，volumeMultiplier 前的基准值）。
const (
	ptsCritical = 25
	ptsMajor    = 12
	ptsMinor    = 5
	ptsInfo     = 1
)

// Counts 是按严重级别统计的问题数量。
type Counts struct {
	Critical int `json:"critical"`
	Major    int `json:"major"`
	Minor    int `json:"minor"`
	Info     int `json:"info"`
}

// Deduction 是一条扣分明细。
type Deduction struct {
	Reason string `json:"reason"`
	Points int    `json:"points"`
	Count  int    `json:"count,omitempty"`
}

// Result 是完整的评分结果。
type Result struct {
	Value           int         `json:"value"`
	Grade           string      `json:"grade"`
	Version         string      `json:"version"`
	Deductions      []Deduction `json:"deductions"`
	ChangedLines    int         `json:"changed_lines,omitempty"`
	FindingsDensity float64     `json:"findings_density,omitempty"`
}

// IsOCRComplete 判断 OCR 状态是否算「完成」。
func IsOCRComplete(status string) bool {
	switch status {
	case OCRStatusSuccess, OCRStatusSkipped, OCRStatusCompletedWithErrors:
		return true
	case "":
		return false
	default:
		return false
	}
}

// Compute 是 severity-v2 评分的唯一计算入口。
//
// 参数：
//   - counts: 各严重级别的问题数
//   - changedLines: 本次变更行数（≤0 时按 100 处理）
//   - warningCount: 需计分的审查警告数（不含 runner_output）
//   - ocrComplete: OCR 是否正常完成
func Compute(counts Counts, changedLines int, warningCount int, ocrComplete bool) Result {
	rawDeduction := counts.Critical*ptsCritical + counts.Major*ptsMajor + counts.Minor*ptsMinor + counts.Info*ptsInfo

	if changedLines <= 0 {
		changedLines = 100
	}

	volumeMultiplier := mathx.Clamp(100.0/float64(changedLines), 0.2, 3.0)

	deductions := make([]Deduction, 0, 6)
	totalSeverityDeduction := 0

	if counts.Critical > 0 {
		pts := int(math.Round(float64(counts.Critical*ptsCritical) * volumeMultiplier))
		deductions = append(deductions, Deduction{Reason: "严重问题", Points: pts, Count: counts.Critical})
		totalSeverityDeduction += pts
	}
	if counts.Major > 0 {
		pts := int(math.Round(float64(counts.Major*ptsMajor) * volumeMultiplier))
		deductions = append(deductions, Deduction{Reason: "高风险问题", Points: pts, Count: counts.Major})
		totalSeverityDeduction += pts
	}
	if counts.Minor > 0 {
		pts := int(math.Round(float64(counts.Minor*ptsMinor) * volumeMultiplier))
		deductions = append(deductions, Deduction{Reason: "中风险问题", Points: pts, Count: counts.Minor})
		totalSeverityDeduction += pts
	}
	if counts.Info > 0 {
		pts := int(math.Round(float64(counts.Info*ptsInfo) * volumeMultiplier))
		deductions = append(deductions, Deduction{Reason: "提示问题", Points: pts, Count: counts.Info})
		totalSeverityDeduction += pts
	}

	value := 100 - totalSeverityDeduction

	if warningCount > 0 {
		points := mathx.Clamp(warningCount*3, 0, 15)
		value -= points
		deductions = append(deductions, Deduction{
			Reason: "OCR 审查警告",
			Points: points,
			Count:  warningCount,
		})
	}

	if !ocrComplete {
		value -= 20
		deductions = append(deductions, Deduction{
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
		deductions = append(deductions, Deduction{
			Reason: "大 PR 低问题密度奖励",
			Points: -5,
			Count:  1,
		})
	}

	value = mathx.Clamp(value, 0, 100)

	return Result{
		Value:           value,
		Grade:           Grade(value),
		Version:         Version,
		Deductions:      deductions,
		ChangedLines:    changedLines,
		FindingsDensity: mathx.Round(density, 2),
	}
}

// Grade 将分数转换为中文等级。
func Grade(value int) string {
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

// CountDiffLines 统计 diff 中的变更行数（+/- 行，不含 hunk header）。
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
