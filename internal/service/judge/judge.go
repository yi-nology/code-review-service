// Package judge 实现 OCR 审查结果的二次综合判断。
//
// OCR binary 负责代码审查（调 LLM 分析 diff，产出 findings）。
// Judge 负责在 OCR 完成后，用 agentkit/llm 对结果做二次分析：
//   - 综合评估 findings 的严重性（是否误报、是否夸大）
//   - 结合项目上下文给出优先级建议
//   - 生成人可读的综合判断摘要
//   - 辅助门禁决策（pass/fail 的置信度）
//
// 调用流程：
//
//	OCR binary → findings + summary
//	    ↓
//	Judge.Analyze(ctx, input, ocrResult) → Judgment（综合判断）
//	    ↓
//	门禁决策 + 写回 PR
package judge

import (
	"context"
	"fmt"
	"strings"

	"github.com/oagentkit/jsonrepair"

	"github.com/yi-nology/code-review-service/internal/ocr"
	"github.com/yi-nology/code-review-service/internal/service/review/domain"

	"github.com/oekit/logx"
)

// Judge 对 OCR 审查结果做二次综合判断。
type Judge struct {
	llm ocr.Provider
}

// NewJudge 构造 Judge。
func NewJudge(llm ocr.Provider) *Judge {
	return &Judge{llm: llm}
}

// Judgment 是综合判断的输出。
type Judgment struct {
	// OverallAssessment 综合评估（人可读，1-3 段文字）
	OverallAssessment string `json:"overall_assessment"`
	// Confidence 对 OCR 结果的置信度（high/medium/low）
	Confidence string `json:"confidence"`
	// RecommendedGate 建议的门禁结果（pass/fail/conditional_pass）
	RecommendedGate string `json:"recommended_gate"`
	// GateReason 门禁建议的理由
	GateReason string `json:"gate_reason"`
	// PrioritizedFindings 按优先级排序的 findings，含误报标记
	PrioritizedFindings []FindingJudgment `json:"prioritized_findings"`
	// SummaryForHuman 面向人的简洁总结（适合 PR 评论）
	SummaryForHuman string `json:"summary_for_human"`
}

// FindingJudgment 是单个 finding 的判断。
type FindingJudgment struct {
	Path     string `json:"path"`
	Line     int    `json:"line,omitempty"`
	Title    string `json:"title"`
	Severity string `json:"severity"`
	// IsLikelyTrue 是否可能是真实问题（false = 可能误报）
	IsLikelyTrue bool `json:"is_likely_true"`
	// Priority 人工处理优先级（1=最高）
	Priority int `json:"priority"`
	// Reason 判断理由
	Reason string `json:"reason"`
}

// AnalyzeInput 是 Judge.Analyze 的输入。
type AnalyzeInput struct {
	// Provider 代码平台（github/gitlab/gitea）
	Provider string
	// Repo 仓库名（owner/name）
	Repo string
	// Diff 代码变更的 diff 文本
	Diff string
	// ChangedFiles 变更文件列表
	ChangedFiles []string
	// ProjectContext 项目上下文（可选，如 README 摘要、语言、框架）
	ProjectContext string
}

// Analyze 对 OCR 审查结果做综合判断。
// 调用 agentkit LLM（带重试/熔断），产出 Judgment。
func (j *Judge) Analyze(ctx context.Context, input AnalyzeInput, ocrResult domain.Result) (*Judgment, error) {
	if j.llm == nil || !j.llm.Available(ctx) {
		// LLM 不可用，返回基于规则的默认判断
		return j.defaultJudgment(ocrResult), nil
	}

	// 构造 prompt
	prompt := buildAnalysisPrompt(input, ocrResult)

	// 调 LLM（走 ocr.Provider → agentkit/llm → eino-openai → API）
	resp, err := j.llm.Complete(ctx, ocr.CompleteRequest{
		SystemPrompt: systemPrompt,
		UserPrompt:   prompt,
		JSONMode:     true,
		MaxTokens:    2000,
	})
	if err != nil {
		logx.Warnf("[judge] LLM analysis failed, using default: %v", err)
		return j.defaultJudgment(ocrResult), nil
	}

	// 解析 LLM 输出
	var judgment Judgment
	if err := jsonrepair.Unmarshal(resp.Content, &judgment); err != nil {
		logx.Warnf("[judge] failed to parse LLM judgment, using default: %v", err)
		return j.defaultJudgment(ocrResult), nil
	}

	// 填充默认值
	if judgment.Confidence == "" {
		judgment.Confidence = "medium"
	}
	if judgment.RecommendedGate == "" {
		judgment.RecommendedGate = defaultGate(ocrResult)
	}
	if judgment.SummaryForHuman == "" {
		judgment.SummaryForHuman = judgment.OverallAssessment
	}
	return &judgment, nil
}

// defaultJudgment 在 LLM 不可用时返回基于规则的默认判断。
func (j *Judge) defaultJudgment(ocrResult domain.Result) *Judgment {
	findings := make([]FindingJudgment, 0, len(ocrResult.Findings))
	for i, f := range ocrResult.Findings {
		findings = append(findings, FindingJudgment{
			Path:         f.Path,
			Line:         f.Line,
			Title:        f.Title,
			Severity:     f.Severity,
			IsLikelyTrue: true,
			Priority:     i + 1,
			Reason:       "自动标记（LLM 二次判断不可用）",
		})
	}
	gate := defaultGate(ocrResult)
	return &Judgment{
		OverallAssessment:   fmt.Sprintf("OCR 完成审查，发现 %d 个问题。", len(ocrResult.Findings)),
		Confidence:          "medium",
		RecommendedGate:     gate,
		GateReason:          fmt.Sprintf("基于 OCR 结果自动判定：%d 个 findings", len(ocrResult.Findings)),
		PrioritizedFindings: findings,
		SummaryForHuman:     buildDefaultSummary(ocrResult),
	}
}

func defaultGate(ocrResult domain.Result) string {
	for _, f := range ocrResult.Findings {
		if f.Severity == "critical" || f.Severity == "major" {
			return "fail"
		}
	}
	return "pass"
}

func buildDefaultSummary(ocrResult domain.Result) string {
	n := len(ocrResult.Findings)
	if n == 0 {
		return "✅ 代码审查通过，未发现问题。"
	}
	critical, major, minor, info := 0, 0, 0, 0
	for _, f := range ocrResult.Findings {
		switch f.Severity {
		case "critical":
			critical++
		case "major":
			major++
		case "minor":
			minor++
		default:
			info++
		}
	}
	parts := []string{fmt.Sprintf("📋 代码审查完成，共发现 **%d** 个问题：", n)}
	if critical > 0 {
		parts = append(parts, fmt.Sprintf("- 🔴 严重: %d", critical))
	}
	if major > 0 {
		parts = append(parts, fmt.Sprintf("- 🟠 高: %d", major))
	}
	if minor > 0 {
		parts = append(parts, fmt.Sprintf("- 🟡 中: %d", minor))
	}
	if info > 0 {
		parts = append(parts, fmt.Sprintf("- 🔵 低: %d", info))
	}
	return strings.Join(parts, "\n")
}
