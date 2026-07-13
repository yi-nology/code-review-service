package mcp

import (
	"fmt"

	mcpserver "github.com/mark3labs/mcp-go/server"
)

// Server wraps mcp-go 的 MCPServer，集中所有 tool 注册入口。
//
// 设计要点（参考 docs/MCP_DESIGN.md）：
//   - 单一 MCPServer 实例，通过 Register*Tools 方法分组注册 tool，
//     避免 9+ 个 tool 全堆在 main.go 里。
//   - 不持有业务状态：所有依赖通过 Register 方法以函数参数注入，
//     方便测试（可传 mock service）和 HTTP 模式复用。
//   - Server 本身不启 transport：HTTP transport 启动由 cmd/mcp-server 决定。
//     原因：跟 transport / 鉴权 / 优雅退出 逻辑解耦,Server 只负责 tool 注册。
type Server struct {
	*mcpserver.MCPServer
}

// ServerConfig 是 NewServer 的依赖,fx 用 struct 注入避免跟 reflect 包的 string 冲突。
//
// fx.Supply(string) 会跟 reflect.makeFuncStub 撞 provider,这里用 struct + 单字段
// 把 name/version 一起喂进去。
type ServerConfig struct {
	Name    string
	Version string
}

// NewServer 构造 MCPServer（fx provider）。
//
// cfg.Name / cfg.Version 出现在 MCP initialize 响应的 serverInfo 字段,
// 客户端会显示给用户/写进日志。
//
// opts 透传给 mcp-go，常见用法：
//   - mcpserver.WithToolCapabilities(false)   // 不需要 listChanged 通知
//   - mcpserver.WithLogger(...)               // 自定义 slog handler
func NewServer(cfg ServerConfig, opts ...mcpserver.ServerOption) *Server {
	return &Server{
		MCPServer: mcpserver.NewMCPServer(cfg.Name, cfg.Version, opts...),
	}
}

// Runtime 暴露底层 *mcpserver.MCPServer 给 HTTP 启动器。
//
// Server 嵌入了 *mcpserver.MCPServer，理论上可以直接 s.MCPServer 暴露出去，
// 但 MCPServer 字段是 unexported（避免外部直接绕过 NewServer 修改内部状态）。
// 显式提供 Runtime() 方法让 transport 选择权放在 main 入口，
// 保持 Server 顶层只负责 tool 注册。
func (s *Server) Runtime() *mcpserver.MCPServer {
	return s.MCPServer
}

// requireString 是 mcp.CallToolRequest.RequireString 的快捷封装，统一 error 风格。
func requireString(args map[string]any, key string) (string, error) {
	v, ok := args[key]
	if !ok {
		return "", fmt.Errorf("%s is required", key)
	}
	s, ok := v.(string)
	if !ok {
		return "", fmt.Errorf("%s must be a string, got %T", key, v)
	}
	return s, nil
}

// optionalString 拿可选 string 字段，缺省返 zero value。
func optionalString(args map[string]any, key string) string {
	v, ok := args[key]
	if !ok {
		return ""
	}
	s, _ := v.(string)
	return s
}
