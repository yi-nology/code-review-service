package llm

import "errors"

// ErrNotConfigured 表示 LLM 未配置或不可用，调用方应降级到非 LLM 逻辑。
var ErrNotConfigured = errors.New("llm not configured or disabled")
