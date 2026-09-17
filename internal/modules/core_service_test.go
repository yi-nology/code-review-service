package modules

import (
	"testing"

	"github.com/yi-nology/code-review-service/internal/config"
	mcpserver "github.com/yi-nology/code-review-service/internal/mcp"

	"github.com/cloudwego/hertz/pkg/app/server"
	"go.uber.org/fx"
)

// TestCoreServiceModule_MCPWiring 防 mcp-server 装配漂移的回归测试。
//
// ReviewService 新增依赖而 CoreServiceModule 未 Provide 时，fx 的静态依赖图
// 校验立即报错——而不是等 mcp-server 部署后启动即 crash（历史上
// GerritClientContainer、*argusclient.Client 两次生产事故都是这个模式）。
// 用 fx.ValidateApp 只校验装配图、不执行 Invokes（NewDatabase 等构造函数
// 需要真实 DB，不能在单测里跑）。
func TestCoreServiceModule_MCPWiring(t *testing.T) {
	cfg := &config.Config{}
	if err := fx.ValidateApp(
		fx.Supply(cfg),
		CoreServiceModule,
		mcpserver.MCPModule,
		fx.NopLogger,
	); err != nil {
		t.Fatalf("CoreServiceModule 无法满足 mcp 装配（ReviewService 新增依赖漏 Provide？）: %v", err)
	}
}

// TestCoreServiceModule_ServerWiring 同理校验主服务的共享装配段：
// CoreServiceModule + 主服务专属扩展 module 的组合必须完整可装配。
// *server.Hertz 在主服务里由 createServer 构造后 Supply（init.go），
// 测试用空实例满足静态图校验（ValidateApp 不执行任何构造函数）。
func TestCoreServiceModule_ServerWiring(t *testing.T) {
	cfg := &config.Config{}
	if err := fx.ValidateApp(
		fx.Supply(cfg, &server.Hertz{}),
		CoreServiceModule,
		WebhookModule,
		HandlerModule,
		ServerModule,
		PreflightModule,
		LLMProbeModule,
		WorkerModule,
		MigrateModule,
		MigrateRulesModule,
		CleanupModule,
		fx.NopLogger,
	); err != nil {
		t.Fatalf("CoreServiceModule 无法满足主服务装配: %v", err)
	}
}
