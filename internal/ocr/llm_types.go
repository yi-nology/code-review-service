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
	Content         string
	InputTokens     int64
	OutputTokens    int64
	CacheReadTokens int64 // prompt cache 命中 token 数（DeepSeek/OpenAI 等支持）
	ReasoningTokens int64 // 推理 token 数（o1/DeepSeek-R1 等推理模型）
	FinishReason    string
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
