package llm

// ProviderPreset 定义 LLM 提供商预设配置
type ProviderPreset struct {
	ID              string        `json:"id"`
	DisplayName     string        `json:"display_name"`
	Category        string        `json:"category"`       // coding_plan, direct, local
	Type            string        `json:"type"`           // openai_compatible, anthropic, gemini, ollama
	BaseURL         string        `json:"base_url"`
	Protocol        string        `json:"protocol"`       // openai, anthropic
	SupportsThinking bool         `json:"supports_thinking"`
	Models          []ModelOption `json:"models"`
	DefaultModel    string        `json:"default_model"`
	Icon            string        `json:"icon"`
	Region          string        `json:"region"`         // cn, global
	RequiresKey     bool          `json:"requires_key"`
}

// ModelOption 定义模型选项
type ModelOption struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
	MaxTokens   int    `json:"max_tokens"`
	SupportsThinking bool `json:"supports_thinking"`
}

// Presets 预设的 LLM 提供商列表
var Presets = []ProviderPreset{
	// ========== Coding Plan (包月套餐) ==========
	{
		ID:          "zhipu-coding-plan",
		DisplayName: "智谱 AI (Coding Plan)",
		Category:    "coding_plan",
		Type:        "openai_compatible",
		BaseURL:     "https://open.bigmodel.cn/api/paas/v4",
		Protocol:    "openai",
		SupportsThinking: true,
		Models: []ModelOption{
			{ID: "glm-5.1", DisplayName: "GLM-5.1", MaxTokens: 4096, SupportsThinking: true},
			{ID: "glm-5.2", DisplayName: "GLM-5.2", MaxTokens: 4096, SupportsThinking: true},
			{ID: "glm-4v-plus", DisplayName: "GLM-4V-Plus", MaxTokens: 4096, SupportsThinking: true},
			{ID: "glm-4-flash", DisplayName: "GLM-4-Flash", MaxTokens: 4096},
		},
		DefaultModel: "glm-5.1",
		Icon:         "zhipu",
		Region:       "cn",
		RequiresKey:  true,
	},
	{
		ID:          "volcengine-coding-plan",
		DisplayName: "火山引擎 (Coding Plan)",
		Category:    "coding_plan",
		Type:        "openai_compatible",
		BaseURL:     "https://ark.cn-beijing.volces.com/api/v3",
		Protocol:    "openai",
		SupportsThinking: true,
		Models: []ModelOption{
			{ID: "doubao-seed-2-0-pro", DisplayName: "豆包 Seed 2.0 Pro", MaxTokens: 4096, SupportsThinking: true},
			{ID: "doubao-seed-2-0-lite", DisplayName: "豆包 Seed 2.0 Lite", MaxTokens: 4096},
			{ID: "doubao-seed-2-0-code", DisplayName: "豆包 Seed 2.0 Code", MaxTokens: 4096, SupportsThinking: true},
		},
		DefaultModel: "doubao-seed-2-0-pro",
		Icon:         "volcengine",
		Region:       "cn",
		RequiresKey:  true,
	},
	{
		ID:          "mimo-coding-plan",
		DisplayName: "小米 MiMo (Coding Plan)",
		Category:    "coding_plan",
		Type:        "openai_compatible",
		BaseURL:     "https://token-plan-cn.xiaomimimo.com/v1",
		Protocol:    "openai",
		SupportsThinking: true,
		Models: []ModelOption{
			{ID: "mimo-v2.5", DisplayName: "MiMo V2.5", MaxTokens: 4096, SupportsThinking: true},
			{ID: "mimo-v2.5-pro", DisplayName: "MiMo V2.5 Pro", MaxTokens: 4096, SupportsThinking: true},
			{ID: "mimo-v2.5-lite", DisplayName: "MiMo V2.5 Lite", MaxTokens: 4096},
		},
		DefaultModel: "mimo-v2.5-pro",
		Icon:         "mimo",
		Region:       "cn",
		RequiresKey:  true,
	},
	{
		ID:          "minimax-coding-plan",
		DisplayName: "MiniMax (Coding Plan)",
		Category:    "coding_plan",
		Type:        "openai_compatible",
		BaseURL:     "https://api.minimax.chat/v1",
		Protocol:    "openai",
		SupportsThinking: false,
		Models: []ModelOption{
			{ID: "MiniMax-M2.5", DisplayName: "MiniMax M2.5", MaxTokens: 4096},
			{ID: "MiniMax-M2.7-highspeed", DisplayName: "MiniMax M2.7 Highspeed", MaxTokens: 4096},
		},
		DefaultModel: "MiniMax-M2.5",
		Icon:         "minimax",
		Region:       "cn",
		RequiresKey:  true,
	},
	{
		ID:          "kimi-coding-plan",
		DisplayName: "Kimi (Coding Plan)",
		Category:    "coding_plan",
		Type:        "openai_compatible",
		BaseURL:     "https://api.moonshot.cn/v1",
		Protocol:    "openai",
		SupportsThinking: false,
		Models: []ModelOption{
			{ID: "moonshot-v1-8k", DisplayName: "Moonshot V1 8K", MaxTokens: 4096},
			{ID: "moonshot-v1-32k", DisplayName: "Moonshot V1 32K", MaxTokens: 4096},
			{ID: "moonshot-v1-128k", DisplayName: "Moonshot V1 128K", MaxTokens: 4096},
		},
		DefaultModel: "moonshot-v1-32k",
		Icon:         "kimi",
		Region:       "cn",
		RequiresKey:  true,
	},

	// ========== Direct API (直接调用) ==========
	{
		ID:          "openai",
		DisplayName: "OpenAI",
		Category:    "direct",
		Type:        "openai_compatible",
		BaseURL:     "https://api.openai.com/v1",
		Protocol:    "openai",
		SupportsThinking: true,
		Models: []ModelOption{
			{ID: "gpt-5.2", DisplayName: "GPT-5.2", MaxTokens: 4096, SupportsThinking: true},
			{ID: "gpt-5.1", DisplayName: "GPT-5.1", MaxTokens: 4096, SupportsThinking: true},
			{ID: "gpt-4o", DisplayName: "GPT-4o", MaxTokens: 4096},
			{ID: "gpt-4o-mini", DisplayName: "GPT-4o Mini", MaxTokens: 4096},
			{ID: "o3", DisplayName: "o3", MaxTokens: 4096, SupportsThinking: true},
			{ID: "o3-mini", DisplayName: "o3 Mini", MaxTokens: 4096, SupportsThinking: true},
			{ID: "o1", DisplayName: "o1", MaxTokens: 4096, SupportsThinking: true},
		},
		DefaultModel: "gpt-5.2",
		Icon:         "openai",
		Region:       "global",
		RequiresKey:  true,
	},
	{
		ID:          "anthropic",
		DisplayName: "Anthropic",
		Category:    "direct",
		Type:        "anthropic",
		BaseURL:     "https://api.anthropic.com",
		Protocol:    "anthropic",
		SupportsThinking: true,
		Models: []ModelOption{
			{ID: "claude-opus-4-5-20250929", DisplayName: "Claude Opus 4.5", MaxTokens: 4096, SupportsThinking: true},
			{ID: "claude-sonnet-4-5-20250929", DisplayName: "Claude Sonnet 4.5", MaxTokens: 4096, SupportsThinking: true},
			{ID: "claude-sonnet-4-20250514", DisplayName: "Claude Sonnet 4", MaxTokens: 4096, SupportsThinking: true},
			{ID: "claude-haiku-4-20250514", DisplayName: "Claude Haiku 4", MaxTokens: 4096, SupportsThinking: true},
		},
		DefaultModel: "claude-sonnet-4-5-20250929",
		Icon:         "anthropic",
		Region:       "global",
		RequiresKey:  true,
	},
	{
		ID:          "deepseek",
		DisplayName: "DeepSeek",
		Category:    "direct",
		Type:        "openai_compatible",
		BaseURL:     "https://api.deepseek.com",
		Protocol:    "openai",
		SupportsThinking: true,
		Models: []ModelOption{
			{ID: "deepseek-v4-pro", DisplayName: "DeepSeek V4 Pro", MaxTokens: 4096, SupportsThinking: true},
			{ID: "deepseek-v4-flash", DisplayName: "DeepSeek V4 Flash", MaxTokens: 4096},
			{ID: "deepseek-chat", DisplayName: "DeepSeek Chat", MaxTokens: 4096},
			{ID: "deepseek-reasoner", DisplayName: "DeepSeek Reasoner", MaxTokens: 4096, SupportsThinking: true},
		},
		DefaultModel: "deepseek-v4-pro",
		Icon:         "deepseek",
		Region:       "cn",
		RequiresKey:  true,
	},
	{
		ID:          "dashscope",
		DisplayName: "阿里百炼",
		Category:    "direct",
		Type:        "openai_compatible",
		BaseURL:     "https://dashscope.aliyuncs.com/compatible-mode/v1",
		Protocol:    "openai",
		SupportsThinking: true,
		Models: []ModelOption{
			{ID: "qwen-max", DisplayName: "Qwen Max", MaxTokens: 4096, SupportsThinking: true},
			{ID: "qwen-plus", DisplayName: "Qwen Plus", MaxTokens: 4096, SupportsThinking: true},
			{ID: "qwen-turbo", DisplayName: "Qwen Turbo", MaxTokens: 4096},
			{ID: "qwen-long", DisplayName: "Qwen Long", MaxTokens: 4096},
			{ID: "qwen-coder-plus", DisplayName: "Qwen Coder Plus", MaxTokens: 4096, SupportsThinking: true},
		},
		DefaultModel: "qwen-plus",
		Icon:         "dashscope",
		Region:       "cn",
		RequiresKey:  true,
	},
	{
		ID:          "zhipu",
		DisplayName: "智谱 AI (直接调用)",
		Category:    "direct",
		Type:        "openai_compatible",
		BaseURL:     "https://open.bigmodel.cn/api/paas/v4",
		Protocol:    "openai",
		SupportsThinking: true,
		Models: []ModelOption{
			{ID: "glm-5.1", DisplayName: "GLM-5.1", MaxTokens: 4096, SupportsThinking: true},
			{ID: "glm-5.2", DisplayName: "GLM-5.2", MaxTokens: 4096, SupportsThinking: true},
			{ID: "glm-4v-plus", DisplayName: "GLM-4V-Plus", MaxTokens: 4096, SupportsThinking: true},
			{ID: "glm-4-flash", DisplayName: "GLM-4-Flash", MaxTokens: 4096},
		},
		DefaultModel: "glm-5.1",
		Icon:         "zhipu",
		Region:       "cn",
		RequiresKey:  true,
	},

	// ========== Local (本地部署) ==========
	{
		ID:          "ollama",
		DisplayName: "Ollama (本地)",
		Category:    "local",
		Type:        "ollama",
		BaseURL:     "http://localhost:11434/v1",
		Protocol:    "openai",
		SupportsThinking: false,
		Models:      []ModelOption{}, // 动态加载
		DefaultModel: "",
		Icon:        "ollama",
		Region:      "global",
		RequiresKey: false,
	},
}

// GetPresetByID 根据 ID 获取预设
func GetPresetByID(id string) *ProviderPreset {
	for i := range Presets {
		if Presets[i].ID == id {
			return &Presets[i]
		}
	}
	return nil
}

// GetPresetsByCategory 根据分类获取预设列表
func GetPresetsByCategory(category string) []ProviderPreset {
	var result []ProviderPreset
	for _, p := range Presets {
		if p.Category == category {
			result = append(result, p)
		}
	}
	return result
}
