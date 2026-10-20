// LLM 工厂：从 OCR settings 构建 agentkit/llm.Generator。
//
// 构建结果统一为 agentkit/llm.Resilient（即使只有一个 Provider），
// 自动获得多模型降级、按模型熔断、预算短路、窗口自适应 fitInput。
// 降级事件和 token 用量通过 OnFallback/OnUsage 回调上报结构化日志。
package ocr

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/oagentkit/llm"
	"github.com/oekit/logx"
)

// BuildGeneratorFromSettings 从 OCR settings 构建 agentkit Generator。
// 返回 nil 表示 LLM 未配置（调用方应降级到 NoopProvider）。
// 统一返回 *llm.Resilient，即使只有一个 Provider——为未来多模型降级预留。
// 自动注册 OnUsage/OnFallback 回调用于结构化日志和指标采集。
func BuildGeneratorFromSettings(ctx context.Context, settings Settings) (llm.Generator, error) {
	data, err := settings.Get(ctx)
	if err != nil {
		return nil, fmt.Errorf("read LLM settings: %w", err)
	}
	if data == nil {
		return nil, nil
	}
	url := strings.TrimSpace(data.URL)
	modelName := strings.TrimSpace(data.ModelName)
	token := strings.TrimSpace(data.AuthToken)
	if url == "" || modelName == "" {
		return nil, nil
	}

	provider, err := llm.NewOpenAIProvider(ctx, llm.OpenAIProviderConfig{
		BaseURL: url,
		APIKey:  token,
		Model:   modelName,
		Timeout: 5 * time.Minute,
	})
	if err != nil {
		return nil, fmt.Errorf("create LLM provider: %w", err)
	}

	// 统一包装为 Resilient：单 Provider 时为直通，未来扩展多 Provider 降级链零改动。
	chain := llm.NewFallbackChain(provider)
	resilient, err := llm.NewResilient(chain, llm.ResilientConfig{})
	if err != nil {
		return nil, fmt.Errorf("create resilient client: %w", err)
	}

	// 注册可观测性回调
	resilient.OnUsage = func(model, stage string, prompt, completion int) {
		logx.Infof("[llm] usage model=%s stage=%s prompt=%d completion=%d total=%d",
			model, stage, prompt, completion, prompt+completion)
	}
	resilient.OnFallback = func(from, to, stage, reason string) {
		logx.Warnf("[llm] fallback stage=%s from=%s to=%s reason=%s",
			stage, from, to, truncateMiddle(reason, 200))
	}

	return resilient, nil
}

// truncateMiddle 截断字符串中间部分，保留头尾。
//
// 按 **rune** 切，不按字节。调用点的入参是 LLM 降级原因——中文占比很高，
// 按字节切会腰斩多字节字符，产出**非法 UTF-8**（落日志、进 JSON、渲染到前端
// 都会出问题）。
//
// maxLen 是**字符数**预算（头尾各占一半），"..." 额外计 3 个字符。
func truncateMiddle(s string, maxLen int) string {
	if maxLen <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	half := maxLen / 2
	if half == 0 {
		// 预算太小，容不下「头 + 省略号 + 尾」，退化为前缀截断。
		return string(runes[:maxLen])
	}
	return string(runes[:half]) + "..." + string(runes[len(runes)-half:])
}
