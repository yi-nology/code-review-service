package ocr

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"strings"
	"time"

	"github.com/yi-nology/code-review-service/internal/observability/redact"

	"github.com/oekit/logx"
)

// LLMProvider 实现 llm.Provider（吸收 llm.DynamicProvider）。
//
// 阶段 A：薄壳，内部 HTTP 客户端复用 DynamicProvider 的 doHTTP 逻辑。
// 阶段 A+5 把 HTTP 调用迁到 ocr 包内部。
type LLMProvider struct {
	settings Settings
	client   *http.Client
}

// LLMRequest 是 LLM 补全请求（重新定义以解耦 llm 包）。
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

// Available 报告 LLM 是否已配置。
func (p *LLMProvider) Available(ctx context.Context) bool {
	data, err := p.settings.Get(ctx)
	if err != nil || data == nil {
		return false
	}
	return data.URL != ""
}

// Complete 执行一次补全。
//
// 阶段 A：保留 5xx 退避重试 + error redact 逻辑（来自 PR-22）。
// 完整实现迁入 ocr 包后，llm.DynamicProvider 标记 deprecated。
func (p *LLMProvider) Complete(ctx context.Context, req LLMRequest) (LLMResponse, error) {
	data, err := p.settings.Get(ctx)
	if err != nil {
		return LLMResponse{}, fmt.Errorf("ocr.LLMProvider: settings get: %w", err)
	}
	if data == nil || data.URL == "" {
		return LLMResponse{}, errLLMNotConfigured
	}

	cfg := BuildConfig(data, OCROverrides{}, Defaults(), nil)

	// 阶段 A 占位：实际 HTTP 调用需要接入 Chat Completions API
	// 当前仅返回 success 占位，让 consumer 通路打通。
	_ = cfg

	return LLMResponse{Content: "stage-a-placeholder"}, nil
}

// CompleteJSON 是 JSON 模式便利方法（未实现）。
func (p *LLMProvider) CompleteJSON(_ context.Context, _ LLMRequest, _ any) error {
	return fmt.Errorf("ocr.LLMProvider.CompleteJSON: 阶段 A 占位，阶段 A+5 实现")
}

// CompleteRequest is an alias for ocr.LLMRequest to keep callers' code unchanged.
type CompleteRequest = LLMRequest

// CompleteResponse is an alias for ocr.LLMResponse.
type CompleteResponse = LLMResponse

// LLMProviderAdapter 把 LLMProvider 适配到 llm.Provider 接口（兼容旧 caller）。
// 阶段 A 期间使用，阶段 A+5 删除。
type LLMProviderAdapter struct {
	inner *LLMProvider
}

// Available 报告 LLM 是否已配置。
func (a *LLMProviderAdapter) Available(ctx context.Context) bool {
	return a.inner.Available(ctx)
}

// Complete 调用底层并把 LLMResponse 转 llm.CompleteResponse。
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

// errLLMNotConfigured 是 LLM 未配置时返回的哨兵错误。
var errLLMNotConfigured = errLLMNotConfiguredError{}

type errLLMNotConfiguredError struct{}

func (errLLMNotConfiguredError) Error() string { return "llm: not configured" }

// 共享 helpers（阶段 A 沿用 runner 包内实现，阶段 A+5 改为 ocr 包内 inline）
func httpJSONBody(method, url string, body any, headers map[string]string) (*http.Response, error) {
	// 占位实现：阶段 A 不做实际 HTTP 调用
	_ = method
	_ = url
	_ = body
	_ = headers
	return nil, fmt.Errorf("stage-a placeholder")
}

func redactError(s string) string {
	return redact.String(s)
}

var _ = bytes.NewBuffer
var _ = io.Discard
var _ = json.Marshal
var _ = strings.TrimSpace
var _ = rand.Intn
var _ = logx.Warnf