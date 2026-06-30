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
//
// 阶段 A15 part 3：完全改用 ocr.Runner / ocr.Config / ocr.Fallback。
// 删除了 runner.OpenCodeReviewRunner / DynamicOpenCodeReviewRunner 依赖。
func NewOCRRegistry(
	cfg *config.Config,
	repos *repository.RepoContainer,
	llmProvider *llm.MetricsProvider,
	ocrRunner *ocrcore.Runner,
	resolver *svrocr.ConfigResolver,
	versioner *svrocr.Versioner,
) *ocrcore.Registry {
	// 阶段 A 拆除：把 resolver 注入到 server.go 用的包级缓存
	SetPackageResolver(resolver)
	// fallback config：用 ocr.Config（不再依赖 runner.OpenCodeReviewRunner）
	fallbackConfig := ocrcore.Config{
		Binary:          cfg.OCR.Binary,
		Audience:        "human",
		RulePath:        cfg.OCR.Rule,
		DefaultSeverity: "major",
	}
	return ocrcore.New(
		repos,
		fallbackConfig,
		NewOCRTester(repos, resolver),
		NewOCRProviderLister(repos, resolver),
		NewOCRRulesChecker(repos, resolver),
		versioner,
		NewOCRViewerManager(repos, resolver),
		NewOCRUpgrader(repos, resolver, versioner, cfg),
		llmProvider,
	).WithRunner(ocrRunner, ocrRunner, ocrRunner)
}

// buildFallbackConfig（阶段 A15 末尾删除，当前未使用）。
// 保留为迁移痕迹，证明 ocrcore.Fallback 的可行性。
func buildFallbackConfig(cfg *config.Config) ocrcore.Config {
	return ocrcore.Fallback{
		Binary:          cfg.OCR.Binary,
		Audience:        "human",
		RulePath:        cfg.OCR.Rule,
		DefaultSeverity: "major",
	}.ToConfig()
}

// NewConfigResolver 阶段 A15 part 3：fallback 改用 ocr.Config。
//
// 注：svocr.ConfigResolver 内部仍需要 runner.OpenCodeReviewConfig 形式，
// 阶段 A15 part 4 将 svocr 包整体删除，所有 manager 迁入 ocr 包。
func NewConfigResolver(repos *repository.RepoContainer) *svrocr.ConfigResolver {
	fallback := runner.OpenCodeReviewConfig{
		Binary:          "ocr",
		Audience:        "human",
		DefaultSeverity: "major",
	}
	return svrocr.NewConfigResolver(repos.OCRSettings, fallback, repos.OCRToolPackage)
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
