package mcp

import (
	"encoding/json"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
)

// 通用 schema helpers：集中定义 tool 的 input/output JSON schema，
// 避免每个 tool 重复写 mcp.WithString / mcp.WithObject。
//
// 用法：
//
//	tool := mcp.NewTool("create_review",
//	    mcp.WithDescription("创建一次代码评审"),
//	    WithStringParam("provider", true, "git 平台标识 (github/gitlab/gitea/gitcode/tencent)"),
//	    WithStringParam("repo", true, "owner/repo"),
//	    WithStringParam("head_sha", true, "head commit SHA"),
//	    WithBoolParam("wait", false, "是否同步等待结果，默认 true"),
//	)

// WithStringParam 等价于 mcp.WithString + Required()/Description()，简化单参数声明。
// required=true 标记 MCP 必填，description 同时给 LLM 与开发者参考。
func WithStringParam(name string, required bool, description string) mcp.ToolOption {
	opts := []mcp.PropertyOption{mcp.Description(description)}
	if required {
		opts = append(opts, mcp.Required())
	}
	return mcp.WithString(name, opts...)
}

// WithIntParam 同 WithStringParam（int 类型）。
func WithIntParam(name string, required bool, description string, defaultVal ...int) mcp.ToolOption {
	opts := []mcp.PropertyOption{mcp.Description(description)}
	if required {
		opts = append(opts, mcp.Required())
	}
	if len(defaultVal) > 0 {
		opts = append(opts, mcp.DefaultNumber(float64(defaultVal[0])))
	}
	return mcp.WithNumber(name, opts...)
}

// WithBoolParam 同 WithStringParam（bool 类型）。
func WithBoolParam(name string, required bool, description string, defaultVal ...bool) mcp.ToolOption {
	opts := []mcp.PropertyOption{mcp.Description(description)}
	if required {
		opts = append(opts, mcp.Required())
	}
	if len(defaultVal) > 0 {
		opts = append(opts, mcp.DefaultBool(defaultVal[0]))
	}
	return mcp.WithBoolean(name, opts...)
}

// WithStringArrayParam 字符串数组参数。
func WithStringArrayParam(name string, required bool, description string) mcp.ToolOption {
	opts := []mcp.PropertyOption{
		mcp.Description(description),
		mcp.Items(map[string]any{"type": "string"}),
	}
	if required {
		opts = append(opts, mcp.Required())
	}
	return mcp.WithArray(name, opts...)
}

// 通用 review 输入 schema（多个 tool 复用：create_review / preview_change_files / scan_repository）。
//
// 字段命名跟 internal/service/review/domain.Request 对齐，避免 tool schema 跟内部类型
// 两套命名 — 减少 LLM/客户端学习成本。
func reviewRequestSchema() []mcp.ToolOption {
	return []mcp.ToolOption{
		WithStringParam("provider", true, "git 平台标识 (github/gitlab/gitea/gitcode/tencent)"),
		WithStringParam("repo", true, "owner/repo"),
		WithStringParam("project_id", false, "项目策略 ID（可选，留空走默认策略）"),
		WithStringParam("pull_request_id", false, "PR 编号（PR 模式必填，GitHub/GitCode/Gitea/Tencent）"),
		WithStringParam("merge_request_iid", false, "MR IID（MR 模式必填，GitLab）"),
		WithStringParam("base_sha", false, "base commit SHA（不传 PR/MR 时必填）"),
		WithStringParam("head_sha", true, "head commit SHA"),
		WithStringArrayParam("requirements", false, "需求/spec/验收标准（多 agent 评审用）"),
		WithStringParam("author", false, "PR 作者登录名（写回评论时署名）"),
		WithStringParam("author_name", false, "PR 作者显示名"),
		WithStringParam("author_avatar", false, "PR 作者头像 URL"),
	}
}

// TextResult 是最常见的成功返回：纯文本内容。
// 用法：return mcp.TextResult("review queued: " + id)
func TextResult(text string) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{mcp.NewTextContent(text)},
	}
}

// JSONResult 把任意结构化数据作为 tool 返回值。
//
// 客户端（minicode / Claude Desktop）通常用两路解析：
//  1. Content[0].Text 给人看（JSON 字符串）
//  2. StructuredContent 给程序（map 已经被序列化好）
func JSONResult(data any) (*mcp.CallToolResult, error) {
	raw, err := json.Marshal(data)
	if err != nil {
		return NewToolErrorf(ErrCategoryInternal, "marshal_failed", "重试",
			"序列化 tool 结果失败: %v", err), nil
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{mcp.NewTextContent(string(raw))},
		StructuredContent: data,
	}, nil
}

// parseTimeoutSec 把 MCP 参数里的 timeout_seconds (float64) 转成 time.Duration，
// 范围 [10s, 600s]，0/负值走默认 180s。create_review / rerun_review 共用。
//
// MCP JSON-RPC 把所有数字解成 float64,跟 ekit config.GetInt 行为一致。
func parseTimeoutSec(v float64) time.Duration {
	const (
		defaultSec = 180
		minSec     = 10
		maxSec     = 600
	)
	if v <= 0 {
		v = defaultSec
	}
	sec := int(v)
	if sec < minSec {
		sec = minSec
	}
	if sec > maxSec {
		sec = maxSec
	}
	return time.Duration(sec) * time.Second
}
