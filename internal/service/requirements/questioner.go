package requirements

import (
	"context"
	"fmt"
	"strings"

	"github.com/yi-nology/code-review-service/internal/service/llm"
	"github.com/cloudwego/hertz/pkg/common/hlog"
)

// ClarifyingQuestion 表示一条需向利益相关者澄清的问题（阶段 4）。
type ClarifyingQuestion struct {
	Category string `json:"category"` // ambiguity | scope | acceptance | edge_case
	Question string `json:"question"`
	Context  string `json:"context"`
}

// GenerateClarifyingQuestions 基于 requirements 歧义、范围边界、缺失验收标准、边缘场景，
// 用 LLM 生成结构化澄清问题（阶段 4）。LLM 不可用时返回空切片（降级）。
func GenerateClarifyingQuestions(ctx context.Context, provider llm.Provider, reqs []string, findings []Finding) []ClarifyingQuestion {
	if provider == nil || !provider.Available(ctx) || len(reqs) == 0 {
		return nil
	}

	systemPrompt := `你是资深技术负责人。基于下述需求与代码评审发现，识别需要向需求方/利益相关者澄清的问题。

关注四类：
- ambiguity：需求措辞含糊或多义
- scope：范围边界不清（是否应包含/排除某功能）
- acceptance：缺失明确的验收标准
- edge_case：未定义的边缘场景行为

只返回 JSON 数组，不要 markdown 围栏或解释。元素格式：
{"category":"ambiguity|scope|acceptance|edge_case","question":"澄清问题","context":"为何需要澄清"}`

	var sb strings.Builder
	sb.WriteString("【需求】\n")
	for i, r := range reqs {
		fmt.Fprintf(&sb, "%d. %s\n", i+1, r)
	}
	if len(findings) > 0 {
		sb.WriteString("\n【评审发现摘要（前 15 条）】\n")
		limit := len(findings)
		if limit > 15 {
			limit = 15
		}
		for i := 0; i < limit; i++ {
			f := findings[i]
			fmt.Fprintf(&sb, "- [%s] %s: %s\n", f.Severity, f.Path, f.Title)
		}
	}
	sb.WriteString("\n请返回澄清问题 JSON 数组。若无值得澄清的问题，返回 []。")

	var questions []ClarifyingQuestion
	if err := llm.CompleteJSON(ctx, provider, llm.CompleteRequest{
		SystemPrompt: systemPrompt,
		UserPrompt:   sb.String(),
		MaxTokens:    2048,
	}, &questions); err != nil {
		hlog.Warnf("generate clarifying questions failed (degraded): %v", err)
		return nil
	}
	return questions
}
