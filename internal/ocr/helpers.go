package ocr

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/oekit/pkg/slicex"
)

// 工具函数：从 runner 包原样迁移，供 config.go 使用。

// errConfig 是 Config 校验错误的工厂函数。
func errConfig(format string, args ...any) error {
	return fmt.Errorf("ocr config: "+format, args...)
}

// firstNonBlank 返回第一个非空白字符串（兼容 runner.firstNonBlank）。
func firstNonBlank(values ...string) string {
	return slicex.First(func(s string) bool { return strings.TrimSpace(s) != "" }, values...)
}

// firstPositive 返回第一个正整数（兼容 runner.firstPositive）。
func firstPositive(values ...int) int {
	return slicex.First(func(v int) bool { return v > 0 }, values...)
}

// formatIntEnv 格式化环境变量。
func formatIntEnv(key string, v int) string {
	return key + "=" + strconv.Itoa(v)
}

// backgroundCtx 返回独立 ctx（用于 tools 查询等与请求 ctx 解耦的场景）。
func backgroundCtx() context.Context {
	return context.Background()
}

// ResolveExtraBody 解析 reasoning extra body 配置。
//
// 阶段 A：从 runner.ResolveExtraBody 迁移；签名保持一致，
// 等阶段 A+1 把 reasoning 逻辑也搬入本包时再替换为内联实现。
func ResolveExtraBody(extraBody, reasoningMode string, reasoningBudget int, provider string) string {
	// 占位实现：阶段 A 仅保持签名一致，完整实现留给 A+1。
	_ = reasoningMode
	_ = reasoningBudget
	_ = provider
	return extraBody
}