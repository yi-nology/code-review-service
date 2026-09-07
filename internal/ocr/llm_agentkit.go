// agentkit 适配层：把 agentkit/llm.Generator 包装为 ocr.Provider 接口。
//
// 迁移策略：
//   - 新实现 AgentKitProvider 实现 ocr.Provider（Complete / Available）
//   - 内部委托 agentkit/llm.Generator（自动获得重试/限速/预算/fitInput/熔断）
//   - 调用方（workflow / discussion / executor）零改动——仍用 ocr.Provider 接口
//   - 旧 LLMProvider（HTTP 手撸）保留作为降级 fallback，待稳定后删除
package ocr

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/oagentkit/llm"

	"github.com/cloudwego/eino/schema"
)

// AgentKitProvider 把 agentkit/llm.Generator 包装为 ocr.Provider。
// 调用方仍用 Complete(ctx, CompleteRequest)，内部走 agentkit 的
// 重试/限速/预算/fitInput/熔断全链路。
type AgentKitProvider struct {
	generator llm.Generator
	settings  Settings // 用于 Available 检查
}

// NewAgentKitProvider 从 agentkit Generator 构造 ocr.Provider。
func NewAgentKitProvider(gen llm.Generator, settings Settings) *AgentKitProvider {
	return &AgentKitProvider{generator: gen, settings: settings}
}

// Available 报告 LLM 是否已配置可用。
func (p *AgentKitProvider) Available(ctx context.Context) bool {
	if p.generator == nil {
		return false
	}
	data, err := p.settings.Get(ctx)
	if err != nil || data == nil {
		return false
	}
	return strings.TrimSpace(data.URL) != "" && strings.TrimSpace(data.ModelName) != ""
}

// Complete 执行一次补全。
// 把 CompleteRequest 转为 eino schema.Message 数组，调 agentkit Generator，
// 再把响应转回 CompleteResponse。
func (p *AgentKitProvider) Complete(ctx context.Context, req CompleteRequest) (CompleteResponse, error) {
	if p.generator == nil {
		return CompleteResponse{}, ErrNotConfigured
	}

	// 构造 eino 消息
	msgs := make([]*schema.Message, 0, 2)
	if strings.TrimSpace(req.SystemPrompt) != "" {
		msgs = append(msgs, &schema.Message{
			Role:    schema.System,
			Content: req.SystemPrompt,
		})
	}
	msgs = append(msgs, &schema.Message{
		Role:    schema.User,
		Content: req.UserPrompt,
	})

	stage := "review"
	if req.JSONMode {
		stage = "review-json"
	}

	// 调 agentkit（带重试/限速/预算/fitInput）
	var out *schema.Message
	var err error
	if req.JSONMode {
		// GenerateJSON 自动处理 JSON 解析失败重试
		var result map[string]any
		err = p.generator.GenerateJSON(ctx, stage, msgs, &result)
		if err == nil {
			// 把 JSON 对象序列化回字符串（保持 CompleteResponse.Content 语义）
			data, marshalErr := json.Marshal(result)
			if marshalErr != nil {
				return CompleteResponse{}, fmt.Errorf("marshal json result: %w", marshalErr)
			}
			out = &schema.Message{Content: string(data)}
		}
	} else {
		out, err = p.generator.Generate(ctx, stage, msgs)
	}

	if err != nil {
		return CompleteResponse{}, err
	}

	// 提取 usage
	resp := CompleteResponse{
		Content: out.Content,
	}
	if out.ResponseMeta != nil && out.ResponseMeta.Usage != nil {
		resp.InputTokens = int64(out.ResponseMeta.Usage.PromptTokens)
		resp.OutputTokens = int64(out.ResponseMeta.Usage.CompletionTokens)
	}
	return resp, nil
}

// 编译期断言
var _ Provider = (*AgentKitProvider)(nil)
