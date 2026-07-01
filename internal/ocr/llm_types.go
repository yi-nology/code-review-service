package ocr

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
)

// =====================================================================
// 从 internal/service/llm 包吸收的核心类型
// 阶段 A+10：消除对 llm 包的 import 依赖
// =====================================================================

// ErrNotConfigured 表示 LLM 未配置或不可用，调用方应降级到非 LLM 逻辑。
var ErrNotConfigured = errors.New("llm not configured or disabled")

// Provider 抽象一次 LLM 补全。
type Provider interface {
	// Complete 执行一次补全。LLM 未配置/不可用时返回 error，调用方应降级。
	Complete(ctx context.Context, req CompleteRequest) (CompleteResponse, error)
	// Available 报告 LLM 是否已配置可用（决定是否启用 LLM 增强阶段）。
	Available(ctx context.Context) bool
}

// OCR 管理接口定义在 domain/ocr_aliases.go。
// ocr 包的 manager 类型（Tester/Versioner/Lister/Checker/Viewer/Upgrader）
// 实现这些 domain 接口，通过 Registry 统一暴露。

// CompleteRequest 描述一次补全请求。
type CompleteRequest struct {
	SystemPrompt string
	UserPrompt   string
	MaxTokens    int  // 0 = 用模型默认
	JSONMode     bool // true = 强制返回 JSON
}

// CompleteResponse 描述一次补全响应。
type CompleteResponse struct {
	Content      string
	InputTokens  int64
	OutputTokens int64
	FinishReason string
}

// CompleteJSON 是便利方法：请求 JSON 输出并把响应解析到 out。
func CompleteJSON(ctx context.Context, p Provider, req CompleteRequest, out any) error {
	req.JSONMode = true
	resp, err := p.Complete(ctx, req)
	if err != nil {
		return err
	}
	return parseJSONLoose(resp.Content, out)
}

// NoopProvider 永远返回 ErrNotConfigured，用于 fx 降级或测试桩。
type NoopProvider struct{}

func NewNoopProvider() NoopProvider { return NoopProvider{} }
func (NoopProvider) Complete(_ context.Context, _ CompleteRequest) (CompleteResponse, error) {
	return CompleteResponse{}, ErrNotConfigured
}
func (NoopProvider) Available(_ context.Context) bool { return false }

// parseJSONLoose 从可能含 markdown 围栏或前后噪声的 LLM 响应中提取并解析 JSON。
func parseJSONLoose(content string, out any) error {
	s := strings.TrimSpace(content)
	if strings.HasPrefix(s, "```") {
		s = strings.TrimPrefix(s, "```")
		if idx := strings.IndexByte(s, '\n'); idx >= 0 {
			first := strings.TrimSpace(s[:idx])
			if first == "json" || first == "" {
				s = s[idx+1:]
			}
		}
		s = strings.TrimSpace(s)
		s = strings.TrimSuffix(s, "```")
		s = strings.TrimSpace(s)
	}
	start := strings.IndexAny(s, "{[")
	end := strings.LastIndexAny(s, "}]")
	if start < 0 || end <= start {
		return json.Unmarshal([]byte(s), out)
	}
	return json.Unmarshal([]byte(s[start:end+1]), out)
}

// ProviderPreset 定义 LLM 提供商预设配置
type ProviderPreset struct {
	ID               string        `json:"id"`
	DisplayName      string        `json:"display_name"`
	Category         string        `json:"category"` // coding_plan, direct, local
	Type             string        `json:"type"`     // openai_compatible, anthropic, gemini, ollama
	BaseURL          string        `json:"base_url"`
	Protocol         string        `json:"protocol"` // openai, anthropic
	SupportsThinking bool          `json:"supports_thinking"`
	Models           []ModelOption `json:"models"`
	DefaultModel     string        `json:"default_model"`
	Icon             string        `json:"icon"`
	Region           string        `json:"region"` // cn, global
	RequiresKey      bool          `json:"requires_key"`
}

// ModelOption 定义模型选项
type ModelOption struct {
	ID             string `json:"id"`
	DisplayName    string `json:"display_name"`
	MaxTokens      int    `json:"max_tokens"`
	ContextWindow  int    `json:"context_window"`
	SupportsJSON   bool   `json:"supports_json"`
	SupportsVision bool   `json:"supports_vision"`
}

// Presets 预设的 LLM 提供商列表
var Presets = []ProviderPreset{
	{
		ID:               "openai",
		DisplayName:      "OpenAI",
		Category:         "direct",
		Type:             "openai_compatible",
		BaseURL:          "https://api.openai.com/v1",
		Protocol:         "openai",
		SupportsThinking: true,
		Models: []ModelOption{
			{ID: "gpt-4o", DisplayName: "GPT-4o", MaxTokens: 16384, ContextWindow: 128000, SupportsJSON: true, SupportsVision: true},
			{ID: "gpt-4o-mini", DisplayName: "GPT-4o Mini", MaxTokens: 16384, ContextWindow: 128000, SupportsJSON: true, SupportsVision: true},
		},
		DefaultModel: "gpt-4o",
		Region:       "global",
		RequiresKey:  true,
	},
	{
		ID:               "anthropic",
		DisplayName:      "Anthropic",
		Category:         "direct",
		Type:             "anthropic",
		BaseURL:          "https://api.anthropic.com/v1",
		Protocol:         "anthropic",
		SupportsThinking: true,
		Models: []ModelOption{
			{ID: "claude-sonnet-4-20250514", DisplayName: "Claude Sonnet 4", MaxTokens: 16384, ContextWindow: 200000, SupportsJSON: true, SupportsVision: true},
			{ID: "claude-opus-4-20250514", DisplayName: "Claude Opus 4", MaxTokens: 16384, ContextWindow: 200000, SupportsJSON: true, SupportsVision: true},
		},
		DefaultModel: "claude-sonnet-4-20250514",
		Region:       "global",
		RequiresKey:  true,
	},
	{
		ID:               "dashscope",
		DisplayName:      "阿里云百炼",
		Category:         "coding_plan",
		Type:             "openai_compatible",
		BaseURL:          "https://dashscope.aliyuncs.com/compatible-mode/v1",
		Protocol:         "openai",
		SupportsThinking: true,
		Models: []ModelOption{
			{ID: "qwen3-coder-plus", DisplayName: "Qwen3 Coder Plus", MaxTokens: 16384, ContextWindow: 262144, SupportsJSON: true},
			{ID: "qwen3-plus", DisplayName: "Qwen3 Plus", MaxTokens: 16384, ContextWindow: 262144, SupportsJSON: true},
			{ID: "qwen3-max", DisplayName: "Qwen3 Max", MaxTokens: 16384, ContextWindow: 262144, SupportsJSON: true},
		},
		DefaultModel: "qwen3-coder-plus",
		Region:       "cn",
		RequiresKey:  true,
	},
	{
		ID:               "z-ai",
		DisplayName:      "智谱 AI",
		Category:         "coding_plan",
		Type:             "openai_compatible",
		BaseURL:          "https://open.bigmodel.cn/api/paas/v4",
		Protocol:         "openai",
		SupportsThinking: true,
		Models: []ModelOption{
			{ID: "glm-4-plus", DisplayName: "GLM-4 Plus", MaxTokens: 4096, ContextWindow: 128000, SupportsJSON: true},
			{ID: "glm-4-flash", DisplayName: "GLM-4 Flash", MaxTokens: 4096, ContextWindow: 128000, SupportsJSON: true},
		},
		DefaultModel: "glm-4-plus",
		Region:       "cn",
		RequiresKey:  true,
	},
	{
		ID:               "deepseek",
		DisplayName:      "DeepSeek",
		Category:         "coding_plan",
		Type:             "openai_compatible",
		BaseURL:          "https://api.deepseek.com/v1",
		Protocol:         "openai",
		SupportsThinking: true,
		Models: []ModelOption{
			{ID: "deepseek-chat", DisplayName: "DeepSeek Chat", MaxTokens: 8192, ContextWindow: 65536, SupportsJSON: true},
			{ID: "deepseek-reasoner", DisplayName: "DeepSeek Reasoner", MaxTokens: 8192, ContextWindow: 65536, SupportsJSON: true},
		},
		DefaultModel: "deepseek-chat",
		Region:       "global",
		RequiresKey:  true,
	},
	{
		ID:               "mimo",
		DisplayName:      "小米 MiMo",
		Category:         "coding_plan",
		Type:             "openai_compatible",
		BaseURL:          "https://api.mimo.ai/v1",
		Protocol:         "openai",
		SupportsThinking: true,
		Models: []ModelOption{
			{ID: "mimo-v2.5-pro", DisplayName: "MiMo v2.5 Pro", MaxTokens: 8192, ContextWindow: 65536, SupportsJSON: true},
		},
		DefaultModel: "mimo-v2.5-pro",
		Region:       "global",
		RequiresKey:  true,
	},
	{
		ID:               "dashscope-openai",
		DisplayName:      "阿里云百炼 (OpenAI 兼容)",
		Category:         "coding_plan",
		Type:             "openai_compatible",
		BaseURL:          "https://dashscope.aliyuncs.com/compatible-mode/v1",
		Protocol:         "openai",
		SupportsThinking: false,
		Models: []ModelOption{
			{ID: "qwen-plus", DisplayName: "Qwen Plus", MaxTokens: 8192, ContextWindow: 131072, SupportsJSON: true},
			{ID: "qwen-turbo", DisplayName: "Qwen Turbo", MaxTokens: 8192, ContextWindow: 131072, SupportsJSON: true},
		},
		DefaultModel: "qwen-plus",
		Region:       "cn",
		RequiresKey:  true,
	},
	{
		ID:               "zhipuai",
		DisplayName:      "智谱 AI (旧)",
		Category:         "coding_plan",
		Type:             "openai_compatible",
		BaseURL:          "https://open.bigmodel.cn/api/paas/v4",
		Protocol:         "openai",
		SupportsThinking: false,
		Models: []ModelOption{
			{ID: "glm-4", DisplayName: "GLM-4", MaxTokens: 4096, ContextWindow: 128000, SupportsJSON: true},
		},
		DefaultModel: "glm-4",
		Region:       "cn",
		RequiresKey:  true,
	},
}
