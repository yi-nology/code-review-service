package llm

import "context"

// NoopProvider 永远返回 ErrNotConfigured，用于 fx 降级或测试桩。
// 当 LLM 未配置时，注入 NoopProvider 可让上层统一走降级路径。
type NoopProvider struct{}

// NewNoopProvider 创建降级用 Provider。
func NewNoopProvider() NoopProvider { return NoopProvider{} }

// Complete 始终返回 ErrNotConfigured。
func (NoopProvider) Complete(_ context.Context, _ CompleteRequest) (CompleteResponse, error) {
	return CompleteResponse{}, ErrNotConfigured
}

// Available 始终返回 false。
func (NoopProvider) Available(_ context.Context) bool { return false }
