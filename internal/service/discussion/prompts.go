package discussion

import (
	"fmt"
	"strings"
)

// llmDiscussionItem 是 LLM 对单条发现的讨论响应（B1）。
type llmDiscussionItem struct {
	FindingID  string  `json:"finding_id"`
	Response   string  `json:"response"`
	Content    string  `json:"content"`
	Evidence   string  `json:"evidence"`
	Confidence float64 `json:"confidence"`
}

// buildDiscourseSystemPrompt 构造讨论的 system prompt。
func buildDiscourseSystemPrompt(agentName string) string {
	return fmt.Sprintf(`你是一名资深代码评审讨论的参与者，评审视角是「%s」。

你将看到其他评审者提交的代码审查发现（含其推理过程）。请从你的专业视角独立判断每条发现。

对每条发现，从以下响应中选择一种：
- AGREE：认同该发现，给出支持理由和具体证据
- CHALLENGE：质疑该发现，说明为何可能是误报、被夸大或不适用
- CONNECT：指出该发现与其他发现的相关性或系统性问题

此外，若讨论过程中你发现新的、之前没人提出的问题，用 SURFACE 提出。

判断原则：
- 只对有把握的发现表态，不必强行对每条都回应
- CHALLENGE 必须给出具体技术理由，而非泛泛而谈
- confidence 反映你的确信程度（0.0-1.0）

必须只返回一个 JSON 数组，不要包含任何 markdown 围栏或解释文字。每个元素格式：
{"finding_id": "<原发现ID>", "response": "AGREE|CHALLENGE|CONNECT", "content": "<你的观点>", "evidence": "<代码或事实依据>", "confidence": 0.0-1.0}

若要 SURFACE 新发现，finding_id 填 "SURFACE"，response 填 "SURFACE"，content 描述新问题、位置与严重程度。`, agentName)
}

// buildDiscourseUserPrompt 构造讨论的 user prompt。
// 仅包含该 agent 未提交的发现（自己不评审自己的发现）。
func buildDiscourseUserPrompt(findings []Finding, agentName string, requirements []string) string {
	var sb strings.Builder

	if len(requirements) > 0 {
		sb.WriteString("【需求上下文】\n")
		for _, r := range requirements {
			sb.WriteString("- ")
			sb.WriteString(r)
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}

	sb.WriteString("【其他评审者的发现】\n")
	count := 0
	for _, f := range findings {
		if containsAgent(f.Agents, agentName) {
			continue
		}
		count++
		fmt.Fprintf(&sb, "\nID: %s\n", f.ID)
		fmt.Fprintf(&sb, "严重程度: %s\n", f.Severity)
		if f.Category != "" {
			fmt.Fprintf(&sb, "类别: %s\n", f.Category)
		}
		if f.EndLine > 0 && f.EndLine != f.Line {
			fmt.Fprintf(&sb, "位置: %s:%d-%d\n", f.Path, f.Line, f.EndLine)
		} else {
			fmt.Fprintf(&sb, "位置: %s:%d\n", f.Path, f.Line)
		}
		fmt.Fprintf(&sb, "内容: %s\n", f.Body)
		if strings.TrimSpace(f.Thinking) != "" {
			fmt.Fprintf(&sb, "评审者推理: %s\n", f.Thinking)
		}
		fmt.Fprintf(&sb, "提交者: %s\n", strings.Join(f.Agents, ", "))
	}

	if count == 0 {
		sb.WriteString("（无需要讨论的发现）\n")
	}

	sb.WriteString("\n请给出你的 JSON 响应数组。")
	return sb.String()
}

// normalizeConfidence 把 LLM 返回的置信度归一化到 [0,1]，非法值降为 0.5。
func normalizeConfidence(c float64) float64 {
	if c < 0 || c > 1 {
		return 0.5
	}
	return c
}
