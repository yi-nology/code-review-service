package writeback

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/yi-nology/code-review-service/internal/service/llm"
	"github.com/yi-nology/code-review-service/internal/service/review"
	"github.com/oekit/logx"
)

// Translator 把多 agent 评审结果改写为拟人化评论（阶段 4）。
type Translator struct {
	provider llm.Provider
}

// NewTranslator 创建拟人化翻译器。provider 不可用时 TranslateToHumanVoice 会降级。
func NewTranslator(provider llm.Provider) *Translator {
	return &Translator{provider: provider}
}

// HumanTranslateResult 是拟人化翻译产物。
type HumanTranslateResult struct {
	Markdown string // 单条第一人称总结评论（可直接作为 PR summary）
	Verdict  string // APPROVE | REQUEST_CHANGES | NEEDS_DISCUSSION
}

// TranslateToHumanVoice 用 LLM 把 findings 改写为第一人称自然语言评论，
// 遵循 Google 代码审查指南（简洁、聚焦代码而非作者、可操作）。
// LLM 不可用时降级为结构化模板格式。
func (t *Translator) TranslateToHumanVoice(ctx context.Context, findings []review.Finding, summary string) (HumanTranslateResult, error) {
	if t.provider == nil || len(findings) == 0 {
		return HumanTranslateResult{Markdown: t.fallbackFormat(findings, summary)}, nil
	}

	// Available() 内部查询 DB，必须带超时，否则连接池耗尽时无限阻塞导致请求卡死
	availCtx, availCancel := context.WithTimeout(ctx, 5*time.Second)
	available := t.provider.Available(availCtx)
	availCancel()
	if !available {
		return HumanTranslateResult{Markdown: t.fallbackFormat(findings, summary)}, nil
	}

	systemPrompt := `你是一名代码评审者，正在以第一人称、自然的语气撰写 PR 评审总结评论。

遵循 Google 代码审查指南：
- 简洁、聚焦代码本身，不评价作者
- 区分"必须修改"（阻断合并）与"建议"（可选）
- 每条问题给出具体、可操作的建议
- 语气友善、专业，像资深同事而非机器
- 用 Markdown，开头给一句话总体结论

只返回 Markdown 评论正文，不要任何解释或围栏。开头标注结论：APPROVE / REQUEST_CHANGES / NEEDS_DISCUSSION 之一。`

	var sb strings.Builder
	sb.WriteString("【评审发现】\n")
	limit := len(findings)
	if limit > 30 {
		limit = 30
	}
	for i := 0; i < limit; i++ {
		f := findings[i]
		fmt.Fprintf(&sb, "- [%s] %s:%d — %s\n", f.Severity, f.Path, f.Line, f.Title)
		if strings.TrimSpace(f.Body) != "" {
			fmt.Fprintf(&sb, "  详情: %s\n", truncateBody(f.Body, 300))
		}
		if strings.TrimSpace(f.SuggestionCode) != "" {
			sb.WriteString("  已附建议代码\n")
		}
	}
	if len(findings) > limit {
		fmt.Fprintf(&sb, "（另有 %d 条次要发现已省略）\n", len(findings)-limit)
	}
	if strings.TrimSpace(summary) != "" {
		fmt.Fprintf(&sb, "\n【原始汇总】%s\n", summary)
	}

	llmCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	resp, err := t.provider.Complete(llmCtx, llm.CompleteRequest{
		SystemPrompt: systemPrompt,
		UserPrompt:   sb.String(),
		MaxTokens:    2048,
	})
	if err != nil {
		logx.Warnf("human translation failed, falling back to template: %v", err)
		return HumanTranslateResult{Markdown: t.fallbackFormat(findings, summary)}, nil
	}

	md := strings.TrimSpace(resp.Content)
	return HumanTranslateResult{
		Markdown: md,
		Verdict:  extractVerdict(md),
	}, nil
}

// fallbackFormat 是降级格式：直接用 Markdown 列出发现（无 LLM 改写）。
func (t *Translator) fallbackFormat(findings []review.Finding, summary string) string {
	var sb strings.Builder
	if strings.TrimSpace(summary) != "" {
		sb.WriteString(summary)
		sb.WriteString("\n\n")
	}
	sb.WriteString("## 评审发现\n\n")
	for _, f := range findings {
		fmt.Fprintf(&sb, "- **[%s]** `%s:%d` — %s\n", f.Severity, f.Path, f.Line, f.Title)
	}
	return sb.String()
}

// extractVerdict 从 Markdown 开头提取结论标记。
func extractVerdict(md string) string {
	upper := strings.ToUpper(md)
	switch {
	case strings.Contains(upper[:min(len(upper), 200)], "APPROVE"):
		return "APPROVE"
	case strings.Contains(upper[:min(len(upper), 200)], "REQUEST_CHANGES"), strings.Contains(upper[:min(len(upper), 200)], "REQUEST CHANGES"):
		return "REQUEST_CHANGES"
	case strings.Contains(upper[:min(len(upper), 200)], "NEEDS_DISCUSSION"), strings.Contains(upper[:min(len(upper), 200)], "NEEDS DISCUSSION"):
		return "NEEDS_DISCUSSION"
	default:
		return ""
	}
}

func truncateBody(s string, max int) string {
	r := []rune(strings.TrimSpace(s))
	if len(r) <= max {
		return string(r)
	}
	return string(r[:max]) + "..."
}
