package ocr

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/yi-nology/code-review-service/internal/httpx"
	"github.com/yi-nology/code-review-service/internal/observability/redact"

	"github.com/oagentkit/breaker"
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
//   - settings 缓存：避免每次 Available/Complete 都读 DB
//   - HTTP 连接池：Transport 配置 MaxIdleConns/IdleConnTimeout
//   - 熔断保护：连续 3 次失败后熔断 5 分钟（agentkit/breaker）
//
// 取代 internal/service/llm/openai.go（282 LOC）。
type LLMProvider struct {
	settings Settings
	client   *http.Client
	breaker  *breaker.Breaker

	// settings 缓存：避免高频热路径（Available/Complete）每次都读 DB
	// 缓存有效期 30s，期间直接返回缓存值
	settingsCacheMu sync.RWMutex
	settingsCache   *llmSettingsCache
}

// settingsCache 缓存 settings.Get() 结果（避免高频热路径如 Available 每次都读 DB）。
// 缓存有效期 30s（OCR 设置变更频率极低）。
type llmSettingsCache struct {
	data     *SettingsData
	expireAt time.Time
}

const llmSettingsCacheTTL = 30 * time.Second

// getSettingsCached 优先从缓存读，缓存过期则回源。
// 线程安全：使用 sync.RWMutex 保护缓存。
func (p *LLMProvider) getSettingsCached(ctx context.Context) (*SettingsData, error) {
	// 快速路径：读锁
	p.settingsCacheMu.RLock()
	cached := p.settingsCache
	if cached != nil && time.Now().Before(cached.expireAt) && cached.data != nil {
		data := cached.data
		p.settingsCacheMu.RUnlock()
		return data, nil
	}
	p.settingsCacheMu.RUnlock()

	// 慢速路径：回源 + 写锁
	data, err := p.settings.Get(ctx)
	if err != nil {
		return nil, err
	}
	p.settingsCacheMu.Lock()
	p.settingsCache = &llmSettingsCache{data: data, expireAt: time.Now().Add(llmSettingsCacheTTL)}
	p.settingsCacheMu.Unlock()
	return data, nil
}

// Reset 清除 settings 缓存。
// 阶段 A+1：settings API 更新后调用此方法强制下次重新加载。
func (p *LLMProvider) Reset() {
	p.settingsCacheMu.Lock()
	p.settingsCache = nil
	p.settingsCacheMu.Unlock()
}

// LLMRequest / LLMResponse 是 CompleteRequest / CompleteResponse 的别名（兼容旧调用方）。
type LLMRequest = CompleteRequest
type LLMResponse = CompleteResponse

// NewLLMProvider 构造 LLMProvider。
func NewLLMProvider(settings Settings) *LLMProvider {
	return &LLMProvider{
		settings: settings,
		client: &http.Client{
			Timeout: 5 * time.Minute,
			Transport: &http.Transport{
				MaxIdleConns:        100, // 默认 100 个空闲连接
				MaxIdleConnsPerHost: 20,  // 每个 host 20 个（OpenAI/Anthropic 共享）
				IdleConnTimeout:     90 * time.Second,
			},
		},
		breaker: breaker.New(3, 5*time.Minute), // 连续 3 次失败熔断，5 分钟后探测恢复
	}
}

// Available 报告 LLM 是否已配置可用。
// 性能优化：通过 settings 缓存避免每次调用都读 DB（30s 缓存有效期）。
func (p *LLMProvider) Available(ctx context.Context) bool {
	data, err := p.getSettingsCached(ctx)
	if err != nil || data == nil {
		return false
	}
	return strings.TrimSpace(data.URL) != "" && strings.TrimSpace(data.ModelName) != ""
}

// Complete 执行一次补全。
// 性能优化：通过 settings 缓存避免每次调用都读 DB（30s 缓存有效期）。
// 熔断保护：连续 3 次 LLM 调用失败后，后续调用快速失败（5 分钟后自动探测恢复）。
func (p *LLMProvider) Complete(ctx context.Context, req LLMRequest) (LLMResponse, error) {
	// 熔断检查
	if p.breaker != nil && !p.breaker.Allow(time.Now()) {
		return LLMResponse{}, fmt.Errorf("LLM 熔断中（连续失败过多，请稍后重试）")
	}

	data, err := p.getSettingsCached(ctx)
	if err != nil {
		if p.breaker != nil {
			p.breaker.Failure(time.Now())
		}
		return LLMResponse{}, fmt.Errorf("读取 LLM 配置失败: %w", err)
	}
	if data == nil {
		return LLMResponse{}, errLLMNotConfigured
	}
	if strings.TrimSpace(data.URL) == "" || strings.TrimSpace(data.ModelName) == "" {
		return LLMResponse{}, errLLMNotConfigured
	}

	var resp LLMResponse
	if data.UseAnthropic {
		resp, err = p.completeAnthropic(ctx, data, req)
	} else {
		resp, err = p.completeOpenAI(ctx, data, req)
	}

	// 熔断状态更新
	if p.breaker != nil {
		if err != nil {
			p.breaker.Failure(time.Now())
		} else {
			p.breaker.Success()
		}
	}
	return resp, err
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
//   - 5xx / 429：最多重试 1 次，退避 1s + 0~250ms jitter (httpx.RetryConfig 默认)
//   - 4xx：不重试
//   - 传输层错误若匹配 isPermanentMessage (DNS 解析失败 / 连接被拒等) 立即返回
//   - 错误消息 redact 脱敏
func (p *LLMProvider) doHTTP(httpReq *http.Request, parse func([]byte) (LLMResponse, error)) (LLMResponse, error) {
	var result LLMResponse
	err := httpx.DoWithRetry(httpReq.Context(), httpx.RetryConfig{}, func(_ context.Context) (bool, error) {
		resp, err := p.client.Do(httpReq)
		if err != nil {
			wrapped := fmt.Errorf("llm http call failed: %w", err)
			// DNS 解析失败 / 连接被拒这类传输层错误不可恢复，重试只会双倍超时。
			if isPermanentMessage(err.Error()) {
				return false, wrapped
			}
			return true, wrapped
		}

		raw, readErr := func() (raw []byte, readErr error) {
			// 用闭包 + defer 保证 ReadAll panic 时 resp.Body 仍被关闭，
			// 且作用域限定在单次重试，不会跨重试累积未关闭连接。
			defer func() { _ = resp.Body.Close() }()
			return io.ReadAll(io.LimitReader(resp.Body, 8<<20))
		}()
		if readErr != nil {
			return true, fmt.Errorf("llm response read failed: %w", readErr)
		}

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			snippet := strings.TrimSpace(string(raw))
			if len(snippet) > 500 {
				snippet = snippet[:500]
			}
			retryable := resp.StatusCode >= 500 || resp.StatusCode == http.StatusTooManyRequests
			wrapped := fmt.Errorf("llm http %d: %s", resp.StatusCode, redact.String(snippet))
			if !retryable {
				return false, wrapped
			}
			logx.Warnf("llm http %d, retrying: %s", resp.StatusCode, wrapped)
			return true, wrapped
		}

		parsed, parseErr := parse(raw)
		if parseErr != nil {
			return false, parseErr
		}
		result = parsed
		return false, nil
	})
	return result, err
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

// LLMProviderAdapter 把 LLMProvider 适配为 Provider 接口。
type LLMProviderAdapter struct {
	inner *LLMProvider
}

// WrapLLMProvider 构造 Adapter。
func WrapLLMProvider(p *LLMProvider) *LLMProviderAdapter {
	return &LLMProviderAdapter{inner: p}
}

// Available 满足 Provider.Available。
func (a *LLMProviderAdapter) Available(ctx context.Context) bool {
	return a.inner.Available(ctx)
}

// Complete 满足 Provider.Complete。
func (a *LLMProviderAdapter) Complete(ctx context.Context, req CompleteRequest) (CompleteResponse, error) {
	resp, err := a.inner.Complete(ctx, req)
	if err != nil {
		return CompleteResponse{}, err
	}
	return resp, nil
}

// errLLMNotConfigured 是 LLM 未配置时返回的哨兵错误。
var errLLMNotConfigured = errors.New("llm: not configured")

// ErrLLMNotConfigured 导出供 llm 包消费者使用。
var ErrLLMNotConfigured = errLLMNotConfigured
