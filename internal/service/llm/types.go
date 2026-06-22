package llm

import (
	"time"
)

// LLMProviderData LLM 提供商数据（用于 API 响应）
type LLMProviderData struct {
	ID              int64         `json:"id"`
	Name            string        `json:"name"`
	DisplayName     string        `json:"display_name"`
	Type            string        `json:"type"`           // openai_compatible, anthropic, gemini, ollama
	BaseURL         string        `json:"base_url"`
	APIKey          string        `json:"api_key,omitempty"` // 只在写入时使用
	Protocol        string        `json:"protocol"`       // openai, anthropic
	AIModel         string        `json:"ai_model"`
	MaxTokens       int           `json:"max_tokens"`
	IsDefault       bool          `json:"is_default"`
	PresetID        string        `json:"preset_id"`
	SupportsThinking bool         `json:"supports_thinking"`
	Models          []ModelOption `json:"models,omitempty"`
	CreatedAt       time.Time     `json:"created_at"`
	UpdatedAt       time.Time     `json:"updated_at"`
}

// LLMProviderCreateRequest 创建提供商请求
type LLMProviderCreateRequest struct {
	Name            string `json:"name" binding:"required"`
	DisplayName     string `json:"display_name"`
	Type            string `json:"type" binding:"required"`
	BaseURL         string `json:"base_url" binding:"required"`
	APIKey          string `json:"api_key"`
	Protocol        string `json:"protocol"`
	AIModel         string `json:"ai_model" binding:"required"`
	MaxTokens       int    `json:"max_tokens"`
	IsDefault       bool   `json:"is_default"`
	PresetID        string `json:"preset_id"`
	SupportsThinking bool   `json:"supports_thinking"`
}

// LLMProviderUpdateRequest 更新提供商请求
type LLMProviderUpdateRequest struct {
	DisplayName     *string `json:"display_name"`
	Type            *string `json:"type"`
	BaseURL         *string `json:"base_url"`
	APIKey          *string `json:"api_key"`
	Protocol        *string `json:"protocol"`
	AIModel         *string `json:"ai_model"`
	MaxTokens       *int    `json:"max_tokens"`
	IsDefault       *bool   `json:"is_default"`
	PresetID        *string `json:"preset_id"`
	SupportsThinking *bool  `json:"supports_thinking"`
}

// LLMProviderListResponse 列表响应
type LLMProviderListResponse struct {
	Items []LLMProviderData `json:"items"`
}

// LLMProviderResponse 单个响应
type LLMProviderResponse struct {
	Provider LLMProviderData `json:"provider"`
}

// LLMTestResponse 测试响应
type LLMTestResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// LLMProviderPresetResponse 预设响应
type LLMProviderPresetResponse struct {
	Presets []ProviderPreset `json:"presets"`
}
