package ocr

import (
	"context"
	"errors"

	"github.com/oekit/pkg/jsonrepair"
)

// =====================================================================
// 从 internal/service/llm 包吸收的核心类型
// 本包不 import 已删除的 llm 包，依赖已彻底解除
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
//
// 解析走 oekit/pkg/jsonrepair：它是「宽容解析」的单一实现——
// 围栏剥离 → 直接解析 → 语法级修复（全角标点 / 非法转义 / 尾逗号 /
// 未闭合括号）→ 从散文中抽对象或数组。
//
// 此前这里是手写的 parseJSONLoose（剥 ``` 围栏 + 取首个 {/[ 到末个 }/]），
// 它只能吃「围栏包裹的纯净 JSON」：模型输出里多一句散文、多个尾逗号、
// 全角冒号，就整段作废——而这些恰恰是 LLM 输出的高频形态。三个调用方
// （discussion / workflow 的 tech-lead 分析 / requirements 澄清提问）
// 都把解析失败当作「降级返回 nil」，等于**静默丢掉整段评审产出**。
func CompleteJSON(ctx context.Context, p Provider, req CompleteRequest, out any) error {
	req.JSONMode = true
	resp, err := p.Complete(ctx, req)
	if err != nil {
		return err
	}
	return jsonrepair.Unmarshal(resp.Content, out)
}

// NoopProvider 永远返回 ErrNotConfigured，用于 fx 降级或测试桩。
type NoopProvider struct{}

func NewNoopProvider() NoopProvider { return NoopProvider{} }
func (NoopProvider) Complete(_ context.Context, _ CompleteRequest) (CompleteResponse, error) {
	return CompleteResponse{}, ErrNotConfigured
}
func (NoopProvider) Available(_ context.Context) bool { return false }
