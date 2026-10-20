package eino

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/oagentkit/llm"
)

// AgentKitChatModel wraps agentkit llm.Generator as an eino ChatModel.
// This provides automatic retry, rate limiting, budget tracking, input window fitting,
// and multi-model failover (when backed by llm.Resilient).
type AgentKitChatModel struct {
	generator llm.Generator
	modelName string // 从 OCR 设置读取的模型名称
}

// NewAgentKitChatModel creates a new ChatModel from agentkit llm.Generator.
// Accepts *llm.Client or *llm.Resilient — both satisfy the Generator interface.
func NewAgentKitChatModel(gen llm.Generator, modelName string) *AgentKitChatModel {
	return &AgentKitChatModel{
		generator: gen,
		modelName: modelName,
	}
}

// Generate implements model.ChatModel interface
func (m *AgentKitChatModel) Generate(ctx context.Context, messages []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	if m.generator == nil {
		return nil, fmt.Errorf("LLM client not configured")
	}

	stage := m.modelName
	if stage == "" {
		stage = "review"
	}

	// Use agentkit's Generate with automatic retry, rate limiting, budget tracking,
	// and multi-model failover (Resilient)
	return m.generator.Generate(ctx, stage, messages)
}

// Stream implements model.ChatModel interface (falls back to non-streaming)
func (m *AgentKitChatModel) Stream(ctx context.Context, messages []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	msg, err := m.Generate(ctx, messages, opts...)
	if err != nil {
		return nil, err
	}

	return schema.StreamReaderFromArray([]*schema.Message{msg}), nil
}

// BindTools implements model.ChatModel interface.
// Tool binding is handled at the eino agent level via ToolsConfig.
func (m *AgentKitChatModel) BindTools(tools []*schema.ToolInfo) error {
	return nil
}

// GenerateJSON generates JSON output with automatic retry on parse failures.
func (m *AgentKitChatModel) GenerateJSON(ctx context.Context, messages []*schema.Message, out interface{}) error {
	if m.generator == nil {
		return fmt.Errorf("LLM client not configured")
	}

	stage := m.modelName
	if stage == "" {
		stage = "review"
	}

	return m.generator.GenerateJSON(ctx, stage, messages, out)
}

// Compile-time interface assertion
var _ model.ChatModel = (*AgentKitChatModel)(nil)
