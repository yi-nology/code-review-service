package modules

import "go.uber.org/fx"

// CoreServiceModule 主服务（init.go）与 mcp-server（cmd/mcp-server）共享的
// ReviewService 依赖装配。
//
// 背景：mcp-server 只拉子集 module，历史上 ReviewService 两次新增依赖
// （GerritClientContainer、*argusclient.Client）时，主服务经 HandlerModule
// 间接拿到、mcp-server 手工拷贝的清单没人 Provide → 启动即 crash。
// 此后 ReviewService 的依赖链变化只改这里，两个入口自动同步；
// cmd/mcp-server 侧另有 TestCoreServiceModule_MCPWiring 防回归。
//
// 顺序约束保持：Database 在最前 → Review 最后。
var CoreServiceModule = fx.Options(
	DatabaseModule,
	RepositoryModule,
	ProviderModule,
	RunnerModule,
	OCRModule,
	LLMModule,
	GerritModule,
	// ArgusClient ReviewService/Worker 的依赖；原 Provide 在 HandlerModule
	// （主服务经它间接获得），上提至此让不拉 Handler 的入口也能满足。
	fx.Provide(NewArgusClient),
	WritebackModule,
	NotificationModule,
	CostModule,
	AgentModule,
	WorkflowModule,
	ReviewModule,
)
