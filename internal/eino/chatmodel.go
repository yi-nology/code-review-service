package eino

import (
	"context"
	"fmt"

	"github.com/oagentkit/llm"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

// AgentKitChatModel wraps agentkit llm.Client as an eino ChatModel.
// This provides automatic retry, rate limiting, budget tracking, and input window fitting.
type AgentKitChatModel struct {
	client    *llm.Client
	modelName string // 从 OCR 设置读取的模型名称
}

// NewAgentKitChatModel creates a new ChatModel from agentkit llm.Client
func NewAgentKitChatModel(client *llm.Client, modelName string) *AgentKitChatModel {
	return &AgentKitChatModel{
		client:    client,
		modelName: modelName,
	}
}

// Generate implements model.ChatModel interface
func (m *AgentKitChatModel) Generate(ctx context.Context, messages []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	if m.client == nil {
		return nil, fmt.Errorf("LLM client not configured")
	}

	// 使用配置的模型名称
	stage := m.modelName
	if stage == "" {
		stage = "review"
	}

	// Use agentkit's Generate with automatic retry, rate limiting, and budget tracking
	return m.client.Generate(ctx, stage, messages)
}

// Stream implements model.ChatModel interface (falls back to non-streaming)
func (m *AgentKitChatModel) Stream(ctx context.Context, messages []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	// Fall back to non-streaming Generate
	msg, err := m.Generate(ctx, messages, opts...)
	if err != nil {
		return nil, err
	}

	return schema.StreamReaderFromArray([]*schema.Message{msg}), nil
}

// BindTools implements model.ChatModel interface
// agentkit llm.Client 不支持在模型级别绑定工具，工具绑定在 eino agent 层处理
func (m *AgentKitChatModel) BindTools(tools []*schema.ToolInfo) error {
	// agentkit llm.Client doesn't support model-level tool binding
	// Tools are bound at the eino agent level via ToolsConfig
	return nil
}

// GenerateJSON generates JSON output with automatic retry on parse failures
func (m *AgentKitChatModel) GenerateJSON(ctx context.Context, messages []*schema.Message, out interface{}) error {
	if m.client == nil {
		return fmt.Errorf("LLM client not configured")
	}

	// 使用配置的模型名称
	stage := m.modelName
	if stage == "" {
		stage = "review"
	}

	return m.client.GenerateJSON(ctx, stage, messages, out)
}

// Compile-time interface assertion
var _ model.ChatModel = (*AgentKitChatModel)(nil)
