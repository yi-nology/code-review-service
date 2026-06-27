package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/yi-nology/code-review-service/internal/repository/postgres/models"
)

// settingsSource 复用 repository 的 OCRSettings 访问（与 runner/dynamic.go 同模式）。
type settingsSource interface {
	Get(ctx context.Context) (*models.OCRSettings, error)
}

// DynamicProvider 按 OCRSettings 动态构造 LLM 调用，配置变更无需重启。
type DynamicProvider struct {
	settings settingsSource
	client   *http.Client
}

// NewDynamicProvider 创建一个复用 OCR 配置的 LLM Provider。
func NewDynamicProvider(settings settingsSource) *DynamicProvider {
	return &DynamicProvider{
		settings: settings,
		client:   &http.Client{Timeout: 5 * time.Minute},
	}
}

// Available 报告 LLM 是否已配置可用。
func (p *DynamicProvider) Available(ctx context.Context) bool {
	s, err := p.settings.Get(ctx)
	if err != nil || s == nil {
		return false
	}
	return s.Enabled && strings.TrimSpace(s.URL) != "" && strings.TrimSpace(s.ModelName) != ""
}

// Complete 执行一次补全。
func (p *DynamicProvider) Complete(ctx context.Context, req CompleteRequest) (CompleteResponse, error) {
	s, err := p.settings.Get(ctx)
	if err != nil {
		return CompleteResponse{}, fmt.Errorf("读取 LLM 配置失败: %w", err)
	}
	if s == nil || !s.Enabled {
		return CompleteResponse{}, ErrNotConfigured
	}
	if strings.TrimSpace(s.URL) == "" || strings.TrimSpace(s.ModelName) == "" {
		return CompleteResponse{}, ErrNotConfigured
	}

	if s.UseAnthropic {
		return p.completeAnthropic(ctx, s, req)
	}
	return p.completeOpenAI(ctx, s, req)
}

// completeOpenAI 走 OpenAI 兼容 chat completions 协议。
func (p *DynamicProvider) completeOpenAI(ctx context.Context, s *models.OCRSettings, req CompleteRequest) (CompleteResponse, error) {
	messages := make([]map[string]string, 0, 2)
	if strings.TrimSpace(req.SystemPrompt) != "" {
		messages = append(messages, map[string]string{"role": "system", "content": req.SystemPrompt})
	}
	messages = append(messages, map[string]string{"role": "user", "content": req.UserPrompt})

	body := map[string]any{
		"model":    s.ModelName,
		"messages": messages,
	}
	if req.MaxTokens > 0 {
		body["max_tokens"] = req.MaxTokens
	}
	if req.JSONMode {
		body["response_format"] = map[string]string{"type": "json_object"}
	}
	if raw := strings.TrimSpace(string(s.ExtraBody)); raw != "" && raw != "null" {
		var extra map[string]any
		if err := json.Unmarshal([]byte(raw), &extra); err == nil {
			for k, v := range extra {
				body[k] = v
			}
		}
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return CompleteResponse{}, fmt.Errorf("marshal openai request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, ensureChatCompletionsPath(s.URL), bytes.NewReader(payload))
	if err != nil {
		return CompleteResponse{}, fmt.Errorf("build openai request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	authHeader := strings.TrimSpace(s.AuthHeader)
	if authHeader == "" {
		httpReq.Header.Set("Authorization", "Bearer "+s.AuthToken)
	} else {
		httpReq.Header.Set(authHeader, s.AuthToken)
	}

	return p.doHTTP(httpReq, func(b []byte) (CompleteResponse, error) {
		var resp openAIResponse
		if err := json.Unmarshal(b, &resp); err != nil {
			return CompleteResponse{}, fmt.Errorf("parse openai response: %w", err)
		}
		if len(resp.Choices) == 0 {
			return CompleteResponse{}, fmt.Errorf("openai response has no choices: %s", string(b))
		}
		out := CompleteResponse{
			Content:      resp.Choices[0].Message.Content,
			InputTokens:  resp.Usage.PromptTokens,
			OutputTokens: resp.Usage.CompletionTokens,
			FinishReason: resp.Choices[0].FinishReason,
		}
		return out, nil
	})
}

// completeAnthropic 走 Anthropic messages 协议。
func (p *DynamicProvider) completeAnthropic(ctx context.Context, s *models.OCRSettings, req CompleteRequest) (CompleteResponse, error) {
	body := map[string]any{
		"model":      s.ModelName,
		"max_tokens": req.MaxTokens,
		"messages":   []map[string]string{{"role": "user", "content": req.UserPrompt}},
	}
	if req.MaxTokens <= 0 {
		body["max_tokens"] = 4096
	}
	if strings.TrimSpace(req.SystemPrompt) != "" {
		body["system"] = req.SystemPrompt
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return CompleteResponse{}, fmt.Errorf("marshal anthropic request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, ensureAnthropicMessagesPath(s.URL), bytes.NewReader(payload))
	if err != nil {
		return CompleteResponse{}, fmt.Errorf("build anthropic request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("anthropic-version", "2023-06-01")
	httpReq.Header.Set("x-api-key", s.AuthToken)

	return p.doHTTP(httpReq, func(b []byte) (CompleteResponse, error) {
		var resp anthropicResponse
		if err := json.Unmarshal(b, &resp); err != nil {
			return CompleteResponse{}, fmt.Errorf("parse anthropic response: %w", err)
		}
		var sb strings.Builder
		for _, block := range resp.Content {
			if block.Type == "text" {
				sb.WriteString(block.Text)
			}
		}
		if sb.Len() == 0 {
			return CompleteResponse{}, fmt.Errorf("anthropic response has no text: %s", string(b))
		}
		return CompleteResponse{
			Content:      sb.String(),
			InputTokens:  resp.Usage.InputTokens,
			OutputTokens: resp.Usage.OutputTokens,
			FinishReason: resp.StopReason,
		}, nil
	})
}

func (p *DynamicProvider) doHTTP(httpReq *http.Request, parse func([]byte) (CompleteResponse, error)) (CompleteResponse, error) {
	resp, err := p.client.Do(httpReq)
	if err != nil {
		return CompleteResponse{}, fmt.Errorf("llm http call failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	raw, readErr := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		snippet := strings.TrimSpace(string(raw))
		if len(snippet) > 500 {
			snippet = snippet[:500]
		}
		return CompleteResponse{}, fmt.Errorf("llm http %d: %s", resp.StatusCode, snippet)
	}
	if readErr != nil {
		return CompleteResponse{}, fmt.Errorf("llm response read failed: %w", readErr)
	}
	return parse(raw)
}

// ensureChatCompletionsPath 兼容两种配置约定：用户可填 base URL（如 .../compatible-mode/v1）
// 也可填完整端点（.../v1/chat/completions）。OCR 二进制使用 base 约定并自行拼接路径，
// 此 Go 客户端需显式补齐 /chat/completions，否则打到 base 路径会收到 "url error" 400。
// 已含该后缀则原样返回（幂等）。
func ensureChatCompletionsPath(raw string) string {
	u := strings.TrimSpace(raw)
	if u == "" || strings.HasSuffix(u, "/chat/completions") {
		return u
	}
	return strings.TrimRight(u, "/") + "/chat/completions"
}

// ensureAnthropicMessagesPath 同理，为 Anthropic messages 协议补齐 /messages。
func ensureAnthropicMessagesPath(raw string) string {
	u := strings.TrimSpace(raw)
	if u == "" || strings.HasSuffix(u, "/messages") {
		return u
	}
	return strings.TrimRight(u, "/") + "/messages"
}

type openAIResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int64 `json:"prompt_tokens"`
		CompletionTokens int64 `json:"completion_tokens"`
	} `json:"usage"`
}

type anthropicResponse struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	StopReason string `json:"stop_reason"`
	Usage      struct {
		InputTokens  int64 `json:"input_tokens"`
		OutputTokens int64 `json:"output_tokens"`
	} `json:"usage"`
}
