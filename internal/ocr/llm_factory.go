// LLM 工厂：从 OCR settings 构建 agentkit/llm.Generator。
//
// 支持 OpenAI 兼容和 Anthropic 两种协议。
// 构建结果为 agentkit/llm.Client（单模型）或 agentkit/llm.Resilient（多模型降级链）。
package ocr

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/oagentkit/llm"

	"github.com/cloudwego/eino/components/model"
)

// BuildGeneratorFromSettings 从 OCR settings 构建 agentkit Generator。
// 返回 nil 表示 LLM 未配置（调用方应降级到 NoopProvider）。
func BuildGeneratorFromSettings(ctx context.Context, settings Settings) (llm.Generator, error) {
	data, err := settings.Get(ctx)
	if err != nil {
		return nil, fmt.Errorf("read LLM settings: %w", err)
	}
	if data == nil {
		return nil, nil
	}
	url := strings.TrimSpace(data.URL)
	model := strings.TrimSpace(data.ModelName)
	token := strings.TrimSpace(data.AuthToken)
	if url == "" || model == "" {
		return nil, nil
	}

	// 根据协议选择 Provider
	protocol := strings.TrimSpace(data.Protocol)
	if protocol == "" {
		if data.UseAnthropic {
			protocol = "anthropic"
		} else {
			protocol = "openai"
		}
	}

	var provider llm.Provider
	switch strings.ToLower(protocol) {
	case "anthropic":
		provider, err = llm.NewOpenAIProvider(ctx, llm.OpenAIProviderConfig{
			BaseURL: url,
			APIKey:  token,
			Model:   model,
			Timeout: 5 * time.Minute,
		})
	default: // openai / openai-responses
		provider, err = llm.NewOpenAIProvider(ctx, llm.OpenAIProviderConfig{
			BaseURL: url,
			APIKey:  token,
			Model:   model,
			Timeout: 5 * time.Minute,
		})
	}
	if err != nil {
		return nil, fmt.Errorf("create LLM provider: %w", err)
	}

	// 构建 agentkit Client（带重试/限速/预算）
	client := llm.NewClient(provider.Model(), model, nil)
	client.MaxRetries = 3
	client.BaseDelay = 2 * time.Second
	client.MaxDelay = 30 * time.Second
	return client, nil
}

// BuildResilientFromSettings 构建多模型降级链（Resilient）。
// 当前只支持单模型；未来可从 DB 读取多个 provider 配置构建降级链。
func BuildResilientFromSettings(ctx context.Context, settings Settings) (llm.Generator, error) {
	gen, err := BuildGeneratorFromSettings(ctx, settings)
	if err != nil || gen == nil {
		return gen, err
	}

	// 包装为 Resilient（即使只有一个 provider，也为未来扩展预留）
	if client, ok := gen.(*llm.Client); ok {
		provider := &singleProvider{
			chatModel: client.RawModel(),
			modelName: client.ModelName,
		}
		chain := llm.NewFallbackChain(provider)
		return llm.NewResilient(chain, llm.ResilientConfig{})
	}
	return gen, nil
}

// singleProvider 把单个 eino ChatModel 包装为 llm.Provider 接口。
type singleProvider struct {
	chatModel model.BaseChatModel
	modelName string
}

func (p *singleProvider) Name() string                          { return p.modelName }
func (p *singleProvider) Model() model.BaseChatModel            { return p.chatModel }
func (p *singleProvider) ModelName() string                     { return p.modelName }
func (p *singleProvider) ContextTokens() int                    { return 128_000 }
func (p *singleProvider) MaxOutputTokens() int                  { return 4096 }
func (p *singleProvider) CostPer1KTokens() (float64, float64)   { return 0, 0 }
