// ask.go argus MCP 通道：经 /mcp（streamable http + Bearer）调用 ask_argus 工具，
// 供「问 Argus」问答代理端点使用。短会话模型：initialize → tools/call → close，
// 不维护长连接（控制台低频问答场景，简单可靠优先）。
package argusclient

import (
	"context"
	"fmt"
	"strings"

	mcpclient "github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/client/transport"
	"github.com/mark3labs/mcp-go/mcp"
)

// askToolName argus 侧问答工具（internal/channel/mcpserver/server.go）。
const askToolName = "ask_argus"

// AskQuestion 基于 argus 对某 PR 的最近审查报告回答追问；question 为空返回
// 完整结论摘要（argus 侧语义）。失败返回 error（含 argus 工具错误文本）。
func (c *Client) AskQuestion(ctx context.Context, platform, owner, repo, number, question string) (string, error) {
	if !c.Enabled() {
		return "", ErrDisabled
	}
	if strings.TrimSpace(platform) == "" || strings.TrimSpace(owner) == "" ||
		strings.TrimSpace(repo) == "" || !isDigits(number) {
		return "", fmt.Errorf("platform/owner/repo 必填且 number 须为纯十进制序号")
	}

	tr, err := transport.NewStreamableHTTP(c.baseURL+"/mcp", transport.WithHTTPHeaders(map[string]string{
		"Authorization": "Bearer " + c.token,
	}))
	if err != nil {
		return "", fmt.Errorf("创建 argus MCP 传输失败: %w", err)
	}
	client := mcpclient.NewClient(tr)
	if err := client.Start(ctx); err != nil {
		return "", fmt.Errorf("启动 argus MCP 会话失败: %w", err)
	}
	defer client.Close()

	initReq := mcp.InitializeRequest{}
	initReq.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	initReq.Params.ClientInfo = mcp.Implementation{Name: "review-service", Version: "1.0"}
	if _, err := client.Initialize(ctx, initReq); err != nil {
		return "", fmt.Errorf("argus MCP 握手失败: %w", err)
	}

	callReq := mcp.CallToolRequest{}
	callReq.Params.Name = askToolName
	callReq.Params.Arguments = map[string]any{
		"platform": strings.ToLower(strings.TrimSpace(platform)),
		"owner":    strings.TrimSpace(owner),
		"repo":     strings.TrimSpace(repo),
		"number":   strings.TrimSpace(number),
		"question": question,
	}
	res, err := client.CallTool(ctx, callReq)
	if err != nil {
		return "", fmt.Errorf("ask_argus 调用失败: %w", err)
	}
	if res == nil {
		return "", fmt.Errorf("ask_argus 返回空结果")
	}
	var texts []string
	for _, item := range res.Content {
		if tc, ok := item.(mcp.TextContent); ok {
			texts = append(texts, tc.Text)
		}
	}
	answer := strings.Join(texts, "\n")
	if res.IsError {
		return "", fmt.Errorf("argus 问答失败: %s", answer)
	}
	if strings.TrimSpace(answer) == "" {
		return "", fmt.Errorf("argus 问答返回空（该 PR 可能尚无审查记录）")
	}
	return answer, nil
}
