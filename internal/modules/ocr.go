package modules

import (
	"github.com/yi-nology/code-review-service/internal/config"
	ocrcore "github.com/yi-nology/code-review-service/internal/ocr"
	"github.com/yi-nology/code-review-service/internal/repository"
	"github.com/yi-nology/code-review-service/internal/service/llm"
	svrocr "github.com/yi-nology/code-review-service/internal/service/ocr"
	"github.com/yi-nology/code-review-service/internal/service/runner"

	"go.uber.org/fx"
)

// OCRModule 提供所有 open-code-review 能力。
//
// B 阶段（当前）：保留原 7 个 fx.Provide 兼容旧 consumer；
// 同时新增 1 个 fx.Provide(ocrcore.New) 让新代码可注入 *ocrcore.Registry。
//
// 阶段 A（后续 PR）：旧 fx.Provide 全部删除，所有 consumer 改用 *ocrcore.Registry。
var OCRModule = fx.Options(
	// 新增：单一入口
	fx.Provide(NewOCRRegistry),

	// 保留：旧 consumer 依赖（含 handler/server/preflight/probe/readiness 等）
	fx.Provide(NewConfigResolver),
	fx.Provide(NewOCRTester),
	fx.Provide(NewOCRProviderLister),
	fx.Provide(NewOCRRulesChecker),
	fx.Provide(NewOCRVersioner),
	fx.Provide(NewOCRViewerManager),
	fx.Provide(NewOCRUpgrader),
)

// NewOCRRegistry 构造 *ocrcore.Registry 单一入口。
// 此函数在阶段 A 期间与原 7 个 NewOCR* 并存；阶段 A 完成 fx 切换后旧函数全部删除。
//
// 副作用：把 resolver 注入到 server.go 用的包级缓存（阶段 A 拆除）。
func NewOCRRegistry(
	cfg *config.Config,
	repos *repository.RepoContainer,
	llmProvider *llm.MetricsProvider,
	dynamicRunner *runner.DynamicOpenCodeReviewRunner,
	fallback *runner.OpenCodeReviewRunner,
	resolver *svrocr.ConfigResolver,
	versioner *svrocr.Versioner,
) *ocrcore.Registry {
	// 阶段 A 拆除：把 resolver 注入到 server.go 用的包级缓存
	SetPackageResolver(resolver)
	return ocrcore.New(
		repos,
		fallback,
		NewOCRTester(repos, resolver),
		NewOCRProviderLister(repos, resolver),
		NewOCRRulesChecker(repos, resolver),
		versioner,
		NewOCRViewerManager(repos, resolver),
		NewOCRUpgrader(repos, resolver, versioner, cfg),
		llmProvider,
	).WithRunner(dynamicRunner, dynamicRunner, dynamicRunner)
}

func NewConfigResolver(repos *repository.RepoContainer, fallback *runner.OpenCodeReviewRunner) *svrocr.ConfigResolver {
	return svrocr.NewConfigResolver(repos.OCRSettings, fallback.Config(), repos.OCRToolPackage)
}

func NewOCRTester(repos *repository.RepoContainer, resolver *svrocr.ConfigResolver) *svrocr.Tester {
	return svrocr.NewTester(repos.OCRSettings, resolver)
}

func NewOCRProviderLister(repos *repository.RepoContainer, resolver *svrocr.ConfigResolver) *svrocr.ProviderLister {
	return svrocr.NewProviderLister(repos.OCRSettings, resolver)
}

func NewOCRRulesChecker(repos *repository.RepoContainer, resolver *svrocr.ConfigResolver) *svrocr.RulesChecker {
	return svrocr.NewRulesChecker(repos.OCRSettings, repos.GlobalRules, resolver)
}

func NewOCRVersioner(repos *repository.RepoContainer, resolver *svrocr.ConfigResolver) *svrocr.Versioner {
	return svrocr.NewVersioner(repos.OCRSettings, resolver)
}

func NewOCRViewerManager(repos *repository.RepoContainer, resolver *svrocr.ConfigResolver) *svrocr.ViewerManager {
	return svrocr.NewViewerManager(repos.OCRSettings, resolver)
}

func NewOCRUpgrader(repos *repository.RepoContainer, resolver *svrocr.ConfigResolver, versioner *svrocr.Versioner, cfg *config.Config) *svrocr.Upgrader {
	return svrocr.NewUpgrader(repos.OCRSettings, resolver, versioner, cfg.OCR.UpgradeInstallDir, cfg.OCR.GitHubRepo)
}
