// Package mcp 暴露 review-service 的能力为 MCP tool（stdio + SSE 两种传输）。
//
// 设计要点（参考 docs/MCP_DESIGN.md，PR-23 引入）：
//  1. 复用 internal/app.ReviewService / GitProviderManager：MCP server 不重新实现业务，
//     而是把 service interface 作为唯一依赖,通过 fx 拉起。
//  2. 独立进程：跟主 HTTP 服务解耦，避免 Hertz 中间件（rate-limit / audit / auth）污染 LLM 调用。
//  3. 错误统一：本文件 NewToolError 把业务错误包成 mcp.CallToolResult{IsError:true, Content:[...]}，
//     而不是 Go error。Go error 在 mcp-go SDK 中会被转成 JSON-RPC INTERNAL_ERROR（code -32603），
//     客户端无法区分「平台未配置」 vs 「LLM 504 超时」 vs 「参数错误」。
//  4. tool 名称 snake_case，匹配 MCP 规范与 Claude Desktop / Cursor / minicode 客户端的惯例。
package mcp

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
)

// 错误类别（写入 StructuredContent.category），供客户端按类别决定是否重试。
const (
	ErrCategoryInvalidParam = "invalid_param"  // 参数缺失/格式错，客户端不应重试
	ErrCategoryNotFound     = "not_found"      // 资源不存在（review_id 不存在等）
	ErrCategoryConflict     = "conflict"       // 状态冲突（重复创建/取消已完成的）
	ErrCategoryUnsupported  = "unsupported"    // provider / feature 未配置
	ErrCategoryUpstream     = "upstream_error" // git 平台 / LLM 失败，可重试
	ErrCategoryInternal     = "internal_error" // DB / 内部错误，联系管理员
	ErrCategoryTimeout      = "timeout"        // 同步等待超时（建议轮询 get_review）
)

// NewToolError 把业务错误包成 MCP tool result。
//
// 返回的 CallToolResult：
//   - IsError=true  → 客户端展示给 LLM
//   - Content[0]    → 人类可读的中文消息
//   - StructuredContent → { category, code, message, suggestion, retryable }
//
// 客户端可以按 category 决定后续动作（例如 ErrCategoryTimeout → 自动转 get_review 轮询）。
func NewToolError(category, code, message, suggestion string, retryable bool) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		IsError: true,
		Content: []mcp.Content{
			mcp.NewTextContent(formatHumanMessage(category, message, suggestion)),
		},
		StructuredContent: map[string]any{
			"category":   category,
			"code":       code,
			"message":    message,
			"suggestion": suggestion,
			"retryable":  retryable,
		},
	}
}

// NewToolErrorf 跟 NewToolError 同语义，message 用 fmt.Sprintf 渲染。
func NewToolErrorf(category, code, suggestion, format string, args ...any) *mcp.CallToolResult {
	return NewToolError(category, code, fmt.Sprintf(format, args...), suggestion,
		category == ErrCategoryUpstream || category == ErrCategoryTimeout)
}

// formatHumanMessage 拼装人类可读消息：「[分类] message（建议: suggestion）」。
// suggestion 为空时省略括号段，保持简洁。
func formatHumanMessage(category, message, suggestion string) string {
	var b strings.Builder
	b.WriteString("[")
	b.WriteString(category)
	b.WriteString("] ")
	b.WriteString(message)
	if suggestion != "" {
		b.WriteString("（建议: ")
		b.WriteString(suggestion)
		b.WriteString("）")
	}
	return b.String()
}

// ClassifyError 把 Go error 翻译到 MCP 错误类别。
//
// 目的：让所有 tool handler 的错误处理走同一条路径，避免每个 handler 自己 if err != nil 然后散落判断。
// 规则（按优先级匹配）：
//  1. context.DeadlineExceeded → ErrCategoryTimeout
//  2. context.Canceled        → ErrCategoryInternal（被取消通常是客户端行为，不应重试）
//  3. 业务错误（sentinel 错误）通过 errors.Is 匹配；其余走兜底 ErrCategoryInternal
//
// 未识别的 err 落到 ErrCategoryInternal + retryable=false（避免 LLM 无限重试无效操作）。
func ClassifyError(err error) *mcp.CallToolResult {
	if err == nil {
		return nil
	}
	msg := err.Error()
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return NewToolError(ErrCategoryTimeout, "ctx_deadline_exceeded",
			"同步等待超时: "+msg,
			"请改用 get_review 按 review_id 轮询结果", true)
	case errors.Is(err, context.Canceled):
		return NewToolError(ErrCategoryInternal, "ctx_canceled",
			"请求被取消: "+msg,
			"如需继续请重新调用", false)
	}
	return NewToolError(ErrCategoryInternal, "internal_error",
		"内部错误: "+msg,
		"请稍后重试或联系管理员", false)
}
