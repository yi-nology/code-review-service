package mcp

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"
)

// 本文件是 stub 占位集合：每个未实现的 tool 返回 notImplemented 错误。
// 真正实现由同名文件（get_review.go / list_reviews.go / …）覆盖 Register* 函数。
//
// 设计：
//   - stub 注册器只调 AddTool 一次；同名文件覆盖 Register* 时，stub 的注册就被 Go 的
//     「不允许重复定义函数」规则挡住。所以同名文件的实现会**替换** stub 的 Register*，
//     而不是两者并存。
//   - 走这套设计是因为 init 顺序不固定（fx.Invoke 拉的是顶层 mcp.RegisterTools 入口），
//     让 stub 跟真实现走同一函数名，最简单。

func notImplemented(name string) func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return NewToolErrorf(ErrCategoryInternal, "not_implemented",
			"该 tool 在当前 mcp-server 版本尚未实现，请升级或反馈",
			"tool %s is not implemented yet", name), nil
	}
}

// 所有 13 个 tool 都已由 get_review.go / findings.go / platform.go 提供。
// stubs.go 现在只保留 notImplemented helper,供未来新增 tool 时临时挂占位。
