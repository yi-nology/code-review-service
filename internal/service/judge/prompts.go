package judge

import (
	"fmt"
	"strings"

	"github.com/yi-nology/code-review-service/internal/service/review/domain"
)

const systemPrompt = `你是一位资深代码审查专家。你的任务是对 OCR（Open Code Review）工具产出的代码审查结果进行二次综合判断。

你的职责：
1. 评估每个 finding 是否可能是真实问题（排除误报）
2. 结合代码变更的整体上下文，判断严重性是否合理
3. 给出面向人类的简洁总结和处理建议
4. 提供门禁决策建议（pass/fail/conditional_pass）

输出要求：严格返回 JSON 格式，不要包含任何其他文本。`

func buildAnalysisPrompt(input AnalyzeInput, ocrResult domain.Result) string {
	var b strings.Builder

	b.WriteString("## 代码审查任务\n\n")
	b.WriteString(fmt.Sprintf("- 平台: %s\n", input.Provider))
	b.WriteString(fmt.Sprintf("- 仓库: %s\n", input.Repo))
	b.WriteString(fmt.Sprintf("- 变更文件: %d 个\n", len(input.ChangedFiles)))
	if input.ProjectContext != "" {
		b.WriteString(fmt.Sprintf("- 项目上下文: %s\n", input.ProjectContext))
	}

	b.WriteString("\n## OCR 审查结果\n\n")
	b.WriteString(fmt.Sprintf("- 审查文件数: %d\n", ocrResult.OCR.FilesReviewed))
	b.WriteString(fmt.Sprintf("- 发现问题数: %d\n", len(ocrResult.Findings)))
	b.WriteString(fmt.Sprintf("- 总 Token: %d\n", ocrResult.OCR.TotalTokens))
	b.WriteString(fmt.Sprintf("- OCR 摘要: %s\n", ocrResult.Summary))

	if len(ocrResult.Findings) > 0 {
		b.WriteString("\n### Findings 列表\n\n")
		for i, f := range ocrResult.Findings {
			b.WriteString(fmt.Sprintf("%d. **[%s]** `%s:%d` — %s\n", i+1, f.Severity, f.Path, f.Line, f.Title))
			if f.Body != "" {
				body := f.Body
				if len(body) > 300 {
					body = body[:300] + "..."
				}
				b.WriteString(fmt.Sprintf("   详情: %s\n", body))
			}
			if f.ExistingCode != "" {
				b.WriteString(fmt.Sprintf("   现有代码: `%s`\n", truncate(f.ExistingCode, 100)))
			}
			if f.SuggestionCode != "" {
				b.WriteString(fmt.Sprintf("   建议代码: `%s`\n", truncate(f.SuggestionCode, 100)))
			}
		}
	}

	b.WriteString("\n## 代码 Diff（前 3000 字符）\n\n")
	diff := input.Diff
	if len(diff) > 3000 {
		diff = diff[:3000] + "\n... (diff 已截断)"
	}
	b.WriteString("```\n")
	b.WriteString(diff)
	b.WriteString("\n```\n")

	b.WriteString("\n## 请输出以下 JSON 格式的综合判断\n\n")
	b.WriteString(`{
  "overall_assessment": "对本次代码变更的综合评估（1-3 段文字，包含变更质量、风险点、优点）",
  "confidence": "high/medium/low（对 OCR 结果的置信度）",
  "recommended_gate": "pass/fail/conditional_pass",
  "gate_reason": "门禁建议的理由",
  "prioritized_findings": [
    {
      "path": "文件路径",
      "line": 行号,
      "title": "问题标题",
      "severity": "critical/major/minor/info",
      "is_likely_true": true/false,
      "priority": 1,
      "reason": "判断理由（为什么是/不是真实问题）"
    }
  ],
  "summary_for_human": "面向人的简洁总结，适合直接贴到 PR 评论中（markdown 格式，包含 emoji）"
}`)

	return b.String()
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// FindingsToInput 从 domain.Result 和 workspace 构造 AnalyzeInput。
func FindingsToInput(provider, repo, diff string, changedFiles []string) AnalyzeInput {
	return AnalyzeInput{
		Provider:     provider,
		Repo:         repo,
		Diff:         diff,
		ChangedFiles: changedFiles,
	}
}
