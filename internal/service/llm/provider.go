// Package llm 提供统一的 LLM 补全抽象（路径 B 地基）。
// discourse / tech-lead 等阶段通过本包调用 LLM；复用 OCR 已有配置，不引入新凭证。
// 任何调用方都必须处理 Complete 返回的 error 并降级到非 LLM 逻辑，保证服务可用。
package llm

import "context"

// Provider 抽象一次 LLM 补全。
type Provider interface {
	// Complete 执行一次补全。LLM 未配置/不可用时返回 error，调用方应降级。
	Complete(ctx context.Context, req CompleteRequest) (CompleteResponse, error)
	// Available 报告 LLM 是否已配置可用（决定是否启用 LLM 增强阶段）。
	Available(ctx context.Context) bool
}

// CompleteRequest 描述一次补全请求。
type CompleteRequest struct {
	SystemPrompt string
	UserPrompt   string
	MaxTokens    int  // 0 = 用模型默认
	JSONMode     bool // true = 强制返回 JSON（OpenAI response_format 或 Anthropic 无对应则忽略）
}

// CompleteResponse 描述一次补全响应。
type CompleteResponse struct {
	Content      string
	InputTokens  int64
	OutputTokens int64
	FinishReason string
}

// CompleteJSON 是便利方法：请求 JSON 输出并把响应解析到 out。
// 任何失败（HTTP/解析/JSON 校验）都返回 error，调用方应降级。
func CompleteJSON(ctx context.Context, p Provider, req CompleteRequest, out any) error {
	req.JSONMode = true
	resp, err := p.Complete(ctx, req)
	if err != nil {
		return err
	}
	return parseJSONLoose(resp.Content, out)
}
