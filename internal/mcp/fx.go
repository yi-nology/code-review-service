package mcp

import (
	"github.com/yi-nology/code-review-service/internal/app"
	gitProvider "github.com/yi-nology/code-review-service/internal/service/provider"
	svcReview "github.com/yi-nology/code-review-service/internal/service/review"

	"go.uber.org/fx"
)

// 服务标识常量，NewServer 用作 serverInfo（出现在 MCP initialize 响应里）。
// 客户端（minicode / Claude Desktop）会显示给用户，应当带 review-service 标识
// 跟 version 号。
const (
	ServerName    = "review-service-mcp"
	ServerVersion = "0.1.0"
)

// MCPModule 是 MCP server 的 fx 模块。
//
// 提供：
//   - *Server（mcp-go MCPServer 包装）
//   - app.ReviewService / app.GitProviderManager interface 适配（fx 不会自动把
//     具体类型当 interface provider,需要显式 Provide 把 *review.Service /
//     *gitProvider.GitProviderService 转成内部 interface）。
//   - fx.Invoke(RegisterTools)：把所有 tool 注册到 server
//
// 不包含：DB / OCR / GitProvider 等基础设施 — 那些由 cmd/mcp-server/main.go
// 在 fx.New 时显式拼装 modules.DatabaseModule / modules.ReviewModule 等。
//
// 为什么不在 MCPModule 里 include 全部 module？
//   - 主服务启动顺序是约定好的（先 DB 再 Repo 再 Runner 再 OCR 再 Review），
//     MCP server 要沿用同一套依赖链。
//   - 把 module 拼装放在 main.go，跨进程复用 fx wiring 时一目了然：
//     主服务和 mcp-server 各自的 main.go 决定拉哪些 module。
//   - 未来加 SSE 模式或者测试场景要换 module 集合时，不需要改 MCPModule。
var MCPModule = fx.Options(
	// fx.Supply 用 struct 一次喂 name/version（避免 string type 跟 reflect 冲突）。
	fx.Supply(ServerConfig{Name: ServerName, Version: ServerVersion}),
	fx.Provide(NewServer),
	// fx interface adapter：把具体类型转成内部 interface，避免 MCPModule
	// 依赖 ReviewModule 暴露 interface（那是污染主服务）。只 mcp server
	// 需要这种语义隔离，单独在这里做最干净。
	fx.Provide(asReviewService),
	fx.Provide(asGitProviderManager),
	fx.Invoke(RegisterTools),
)

// asReviewService 把 *review.Service 适配为 app.ReviewService interface。
//
// fx 不会自动把具体类型满足的 interface 注册进容器；要让 MCPModule.RegisterTools
// 拿 app.ReviewService 而非 *review.Service，必须显式 Provide。
func asReviewService(s *svcReview.Service) app.ReviewService { return s }

// asGitProviderManager 把 *gitProvider.GitProviderService 适配为 app.GitProviderManager interface。
func asGitProviderManager(s *gitProvider.GitProviderService) app.GitProviderManager { return s }

// RegisterTools 是 fx Invoke 入口，依赖 service / git manager 就绪后注册所有 tool。
//
// 为什么用 fx.Invoke 而不是 server 自己的 fx hook？
//   - fx.Invoke 会在所有 Provide 完成、OnStart 之后执行；此时所有 service
//     都已 wire 好，Register 内部能安全使用。
//   - server 自身不依赖 fx，可以独立测试（传 mock service 即可）。
func RegisterTools(s *Server, svc app.ReviewService, git app.GitProviderManager) {
	RegisterReviewTools(s, svc)
	RegisterPlatformTools(s, git)
}

// RegisterReviewTools 注册 Tier 1 评审核心 9 个 tool。
func RegisterReviewTools(s *Server, svc app.ReviewService) {
	RegisterCreateReview(s, svc)
	RegisterGetReview(s, svc)
	RegisterListReviews(s, svc)
	RegisterCancelReview(s, svc)
	RegisterRerunReview(s, svc)
	RegisterGetReviewFindings(s, svc)
	RegisterGetSuggestionPatch(s, svc)
	RegisterPreviewChangeFiles(s, svc)
	RegisterScanRepository(s, svc)
}

// RegisterPlatformTools 注册 Tier 2 平台辅助 4 个 tool。
func RegisterPlatformTools(s *Server, git app.GitProviderManager) {
	RegisterListProviderRepos(s, git)
	RegisterListPullRequests(s, git)
	RegisterGetBranchSHA(s, git)
	RegisterGetPullRequest(s, git)
}
