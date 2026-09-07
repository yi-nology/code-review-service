package eino

import (
	"context"
	"fmt"

	"github.com/oagentkit/llm"
	"github.com/yi-nology/code-review-service/internal/service/agent"
	"github.com/yi-nology/code-review-service/internal/service/review/domain"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

// ReviewADKAgentConfig contains configuration for the review ADK agent
type ReviewADKAgentConfig struct {
	// Name of the agent
	Name string
	// Description of the agent's capabilities
	Description string
	// Instruction used as the system prompt
	Instruction string
	// LLMClient for the agent's model (from agentkit)
	LLMClient *llm.Client
	// Tools available to the agent
	Tools []tool.BaseTool
	// AgentManager for multi-agent coordination
	AgentManager *agent.AgentManager
}

// ReviewADKAgent wraps an eino ADK agent for code review
type ReviewADKAgent struct {
	agent  *adk.ChatModelAgent
	config ReviewADKAgentConfig
}

// NewReviewADKAgent creates a new review ADK agent
func NewReviewADKAgent(ctx context.Context, config ReviewADKAgentConfig) (*ReviewADKAgent, error) {
	if config.LLMClient == nil {
		return nil, fmt.Errorf("LLM client is required")
	}

	// Create ChatModel wrapper using agentkit
	chatModel := NewAgentKitChatModel(config.LLMClient, config.Name)

	// Configure tools
	toolsConfig := adk.ToolsConfig{}
	if len(config.Tools) > 0 {
		toolsConfig.Tools = config.Tools
	}

	// Create ADK agent
	agentConfig := &adk.ChatModelAgentConfig{
		Name:        config.Name,
		Description: config.Description,
		Instruction: config.Instruction,
		Model:       chatModel,
		ToolsConfig: toolsConfig,
	}

	adkAgent, err := adk.NewChatModelAgent(ctx, agentConfig)
	if err != nil {
		return nil, fmt.Errorf("create ADK agent: %w", err)
	}

	return &ReviewADKAgent{
		agent:  adkAgent,
		config: config,
	}, nil
}

// Run executes the ADK agent with the given input
func (a *ReviewADKAgent) Run(ctx context.Context, input string) (string, error) {
	// Create initial message
	messages := []*schema.Message{
		{
			Role:    schema.User,
			Content: input,
		},
	}

	// Run the agent using the Agent interface
	agentInput := &adk.AgentInput{
		Messages: messages,
	}

	// Get the async iterator
	iter := a.agent.Run(ctx, agentInput)
	if iter == nil {
		return "", fmt.Errorf("agent returned nil iterator")
	}

	// Collect all events using Next()
	var result string
	for {
		event, ok := iter.Next()
		if !ok {
			break
		}
		if event.Err != nil {
			return "", fmt.Errorf("agent error: %w", event.Err)
		}
		if event.Output != nil && event.Output.MessageOutput != nil && event.Output.MessageOutput.Message != nil {
			result = event.Output.MessageOutput.Message.Content
		}
	}

	if result == "" {
		return "", fmt.Errorf("agent produced no output")
	}

	return result, nil
}

// RunWithReviewContext runs the agent with review-specific context
func (a *ReviewADKAgent) RunWithReviewContext(ctx context.Context, reviewID string, req domain.Request, workspace domain.Workspace) (*ReviewADKOutput, error) {
	// Build context input
	input := fmt.Sprintf(`Review ID: %s
Provider: %s
Repository: %s
Base SHA: %s
Head SHA: %s
Workspace: %s

Please analyze the code changes and provide findings.`,
		reviewID, req.Provider, req.Repo, req.BaseSHA, req.HeadSHA, workspace.Dir)

	result, err := a.Run(ctx, input)
	if err != nil {
		return nil, err
	}

	return &ReviewADKOutput{
		ReviewID: reviewID,
		Summary:  result,
	}, nil
}

// ReviewADKOutput contains the output from the ADK agent
type ReviewADKOutput struct {
	ReviewID string
	Summary  string
	Findings []domain.Finding
}

// DefaultReviewInstruction returns the default instruction for the review agent
func DefaultReviewInstruction() string {
	return `You are an expert code reviewer. Your task is to analyze code changes and provide detailed findings.

When reviewing code:
1. Identify potential bugs, security issues, and performance problems
2. Check for code style and best practices
3. Suggest improvements and optimizations
4. Provide clear, actionable feedback

Output format:
- For each finding, include: path, line number, severity (critical/major/minor/info), title, and description
- Group findings by category
- Provide an overall summary of the review`
}

// CreateReviewTools creates the standard tools for a review agent
func CreateReviewTools(runner domain.Runner, provider domain.Provider, writeback domain.WritebackPublisher) ([]tool.BaseTool, error) {
	ocrTool, err := NewOCRReviewTool(runner)
	if err != nil {
		return nil, fmt.Errorf("create ocr tool: %w", err)
	}

	providerTool, err := NewProviderTool(provider)
	if err != nil {
		return nil, fmt.Errorf("create provider tool: %w", err)
	}

	writebackTool, err := NewWritebackTool(writeback)
	if err != nil {
		return nil, fmt.Errorf("create writeback tool: %w", err)
	}

	return []tool.BaseTool{
		ocrTool,
		providerTool,
		writebackTool,
	}, nil
}
