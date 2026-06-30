package ocr

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"strings"
	"time"

	"github.com/yi-nology/code-review-service/internal/observability/redact"

	"github.com/oekit/logx"
)

// LLMProvider 实现 Provider / Complete 接口（吸收 llm.DynamicProvider）。
//
// 阶段 A11 完整化：
//   - OpenAI 兼容 chat completions 协议
//   - Anthropic messages 协议（UseAnthropic=true 切换）
//   - 5xx / 429 自动重试 1 次（指数退避 1s + jitter）
//   - 4xx 不重试
//   - 错误消息 redact 脱敏（Bearer / api_key / sk- / ghp_ 等）
//
// 取代 internal/service/llm/openai.go（282 LOC）。
type LLMProvider struct {
	settings Settings
	client   *http.Client
}

// LLMRequest 是 LLM 补全请求。
type LLMRequest struct {
	SystemPrompt string
	UserPrompt   string
	MaxTokens    int
	JSONMode     bool
}

// LLMResponse 是 LLM 补全响应。
type LLMResponse struct {
	Content      string
	InputTokens  int64
	OutputTokens int64
	FinishReason string
}

// NewLLMProvider 构造 LLMProvider。
func NewLLMProvider(settings Settings) *LLMProvider {
	return &LLMProvider{
		settings: settings,
		client:   &http.Client{Timeout: 5 * time.Minute},
	}
}

// Available 报告 LLM 是否已配置可用。
func (p *LLMProvider) Available(ctx context.Context) bool {
	data, err := p.settings.Get(ctx)
	if err != nil || data == nil {
		return false
	}
	return strings.TrimSpace(data.URL) != "" && strings.TrimSpace(data.ModelName) != ""
}

// Complete 执行一次补全。
func (p *LLMProvider) Complete(ctx context.Context, req LLMRequest) (LLMResponse, error) {
	data, err := p.settings.Get(ctx)
	if err != nil {
		return LLMResponse{}, fmt.Errorf("读取 LLM 配置失败: %w", err)
	}
	if data == nil {
		return LLMResponse{}, errLLMNotConfigured
	}
	if strings.TrimSpace(data.URL) == "" || strings.TrimSpace(data.ModelName) == "" {
		return LLMResponse{}, errLLMNotConfigured
	}

	if data.UseAnthropic {
		return p.completeAnthropic(ctx, data, req)
	}
	return p.completeOpenAI(ctx, data, req)
}

// completeOpenAI 走 OpenAI 兼容 chat completions 协议。
func (p *LLMProvider) completeOpenAI(ctx context.Context, s *SettingsData, req LLMRequest) (LLMResponse, error) {
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
	if raw := strings.TrimSpace(s.ExtraBody); raw != "" && raw != "null" {
		var extra map[string]any
		if err := json.Unmarshal([]byte(raw), &extra); err == nil {
			for k, v := range extra {
				body[k] = v
			}
		}
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return LLMResponse{}, fmt.Errorf("marshal openai request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, ensureChatCompletionsPath(s.URL), bytes.NewReader(payload))
	if err != nil {
		return LLMResponse{}, fmt.Errorf("build openai request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if strings.TrimSpace(s.AuthHeader) == "" {
		httpReq.Header.Set("Authorization", "Bearer "+s.AuthToken)
	} else {
		httpReq.Header.Set(s.AuthHeader, s.AuthToken)
	}

	return p.doHTTP(httpReq, func(b []byte) (LLMResponse, error) {
		var resp openAIResponse
		if err := json.Unmarshal(b, &resp); err != nil {
			return LLMResponse{}, fmt.Errorf("parse openai response: %w", err)
		}
		if len(resp.Choices) == 0 {
			return LLMResponse{}, fmt.Errorf("openai response has no choices: %s", string(b))
		}
		return LLMResponse{
			Content:      resp.Choices[0].Message.Content,
			InputTokens:  resp.Usage.PromptTokens,
			OutputTokens: resp.Usage.CompletionTokens,
			FinishReason: resp.Choices[0].FinishReason,
		}, nil
	})
}

// completeAnthropic 走 Anthropic messages 协议。
func (p *LLMProvider) completeAnthropic(ctx context.Context, s *SettingsData, req LLMRequest) (LLMResponse, error) {
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
		return LLMResponse{}, fmt.Errorf("marshal anthropic request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, ensureAnthropicMessagesPath(s.URL), bytes.NewReader(payload))
	if err != nil {
		return LLMResponse{}, fmt.Errorf("build anthropic request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("anthropic-version", "2023-06-01")
	httpReq.Header.Set("x-api-key", s.AuthToken)

	return p.doHTTP(httpReq, func(b []byte) (LLMResponse, error) {
		var resp anthropicResponse
		if err := json.Unmarshal(b, &resp); err != nil {
			return LLMResponse{}, fmt.Errorf("parse anthropic response: %w", err)
		}
		var sb strings.Builder
		for _, block := range resp.Content {
			if block.Type == "text" {
				sb.WriteString(block.Text)
			}
		}
		if sb.Len() == 0 {
			return LLMResponse{}, fmt.Errorf("anthropic response has no text: %s", string(b))
		}
		return LLMResponse{
			Content:      sb.String(),
			InputTokens:  resp.Usage.InputTokens,
			OutputTokens: resp.Usage.OutputTokens,
			FinishReason: resp.StopReason,
		}, nil
	})
}

// doHTTP 执行 LLM HTTP 请求。
//
// 行为：
//   - 5xx / 429：最多重试 1 次，指数退避 1s + jitter
//   - 4xx：不重试
//   - 错误消息 redact 脱敏
func (p *LLMProvider) doHTTP(httpReq *http.Request, parse func([]byte) (LLMResponse, error)) (LLMResponse, error) {
	const maxAttempts = 2
	var lastResp LLMResponse
	var lastErr error

	for attempt := 0; attempt < maxAttempts; attempt++ {
		if attempt > 0 {
			backoff := time.Second + time.Duration(rand.Intn(250))*time.Millisecond
			select {
			case <-httpReq.Context().Done():
				return LLMResponse{}, httpReq.Context().Err()
			case <-time.After(backoff):
			}
		}

		resp, err := p.client.Do(httpReq)
		if err != nil {
			lastErr = fmt.Errorf("llm http call failed: %w", err)
			continue
		}

		raw, readErr := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
		_ = resp.Body.Close()

		if readErr != nil {
			lastErr = fmt.Errorf("llm response read failed: %w", readErr)
			continue
		}

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			snippet := strings.TrimSpace(string(raw))
			if len(snippet) > 500 {
				snippet = snippet[:500]
			}
			retryable := resp.StatusCode >= 500 || resp.StatusCode == http.StatusTooManyRequests
			lastErr = fmt.Errorf("llm http %d: %s", resp.StatusCode, redact.String(snippet))
			if !retryable {
				return LLMResponse{}, lastErr
			}
			logx.Warnf("llm http %d (attempt %d/%d), retrying: %s", resp.StatusCode, attempt+1, maxAttempts, lastErr)
			continue
		}

		return parse(raw)
	}
	return lastResp, lastErr
}

// ensureChatCompletionsPath 兼容两种配置约定。
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

// CompleteJSON 是 JSON 模式便利方法。
func (p *LLMProvider) CompleteJSON(ctx context.Context, req LLMRequest, out any) error {
	req.JSONMode = true
	resp, err := p.Complete(ctx, req)
	if err != nil {
		return err
	}
	return json.Unmarshal([]byte(resp.Content), out)
}

// LLMProviderAdapter 把 LLMProvider 适配到 llm.Provider 接口（兼容旧 caller）。
type LLMProviderAdapter struct {
	inner *LLMProvider
}

// WrapLLMProvider 构造 Adapter。
func WrapLLMProvider(p *LLMProvider) *LLMProviderAdapter {
	return &LLMProviderAdapter{inner: p}
}

// Available 报告 LLM 是否已配置。
func (a *LLMProviderAdapter) Available(ctx context.Context) bool {
	return a.inner.Available(ctx)
}

// Complete 转发到内部 LLMProvider。
func (a *LLMProviderAdapter) Complete(ctx context.Context, req CompleteRequest) (CompleteResponse, error) {
	resp, err := a.inner.Complete(ctx, LLMRequest{
		SystemPrompt: req.SystemPrompt,
		UserPrompt:   req.UserPrompt,
		MaxTokens:    req.MaxTokens,
		JSONMode:     req.JSONMode,
	})
	if err != nil {
		return CompleteResponse{}, err
	}
	return CompleteResponse{
		Content:      resp.Content,
		InputTokens:  resp.InputTokens,
		OutputTokens: resp.OutputTokens,
		FinishReason: resp.FinishReason,
	}, nil
}

// CompleteJSON 转发到内部 LLMProvider。
func (a *LLMProviderAdapter) CompleteJSON(ctx context.Context, req CompleteRequest, out any) error {
	return a.inner.CompleteJSON(ctx, LLMRequest{
		SystemPrompt: req.SystemPrompt,
		UserPrompt:   req.UserPrompt,
		MaxTokens:    req.MaxTokens,
		JSONMode:     req.JSONMode,
	}, out)
}

// CompleteRequest / CompleteResponse 是 llm 包的兼容 type alias。
type (
	CompleteRequest = LLMRequest
	CompleteResponse = LLMResponse
)

// errLLMNotConfigured 是 LLM 未配置时返回的哨兵错误。
var errLLMNotConfigured = errors.New("llm: not configured")

// ErrLLMNotConfigured 导出供 llm 包消费者使用。
var ErrLLMNotConfigured = errLLMNotConfigured

// 强制使用（防 unused import）
var _ = rand.Intn
var _ = bytes.NewReader
var _ = json.Marshal
var _ = http.MethodPost
var _ = io.Discard