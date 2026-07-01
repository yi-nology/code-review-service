package modules

import (
	"github.com/yi-nology/code-review-service/internal/config"
	ocrcore 	"github.com/yi-nology/code-review-service/internal/ocr"
	"github.com/yi-nology/code-review-service/internal/repository"
	"github.com/yi-nology/code-review-service/internal/service/llm"

	"go.uber.org/fx"
)

// OCRModule 提供所有 open-code-review 能力。
//
// 阶段 A15：完全改用 ocrcore.*（替代 svocr.* + runner.OpenCodeReviewConfig）。
var OCRModule = fx.Options(
	// 单一入口
	fx.Provide(NewOCRRegistry),

	// 旧 7 个 fx.Provide 阶段 A15 末尾删除
	fx.Provide(NewConfigResolver),
	fx.Provide(NewOCRTester),
	fx.Provide(NewOCRProviderLister),
	fx.Provide(NewOCRRulesChecker),
	fx.Provide(NewOCRVersioner),
	fx.Provide(NewOCRViewerManager),
	fx.Provide(NewOCRUpgrader),
)

// NewOCRRegistry 构造 *ocrcore.Registry 单一入口。
func NewOCRRegistry(
	cfg *config.Config,
	repos *repository.RepoContainer,
	llmProvider *llm.MetricsProvider,
	ocrRunner *ocrcore.Runner,
	resolver *ocrcore.ConfigResolver,
	versioner *ocrcore.Versioner,
) *ocrcore.Registry {
	SetPackageResolver(resolver)
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

// NewConfigResolver 阶段 A15 part 6：fallback 改用 ocr.Fallback。
func NewConfigResolver(repos *repository.RepoContainer) *ocrcore.ConfigResolver {
	fallback := ocrcore.Fallback{
		Binary:          "ocr",
		Audience:        "human",
		DefaultSeverity: "major",
	}
	return ocrcore.NewConfigResolver(
		ocrcore.NewSettingsAdapter(repos.OCRSettings),
		fallback,
		nil, // TODO: implement ocr.ToolSettings adapter for repos.OCRToolPackage
	)
}

func NewOCRTester(repos *repository.RepoContainer, resolver *ocrcore.ConfigResolver) *ocrcore.Tester {
	return ocrcore.NewTester(ocrcore.NewSettingsAdapter(repos.OCRSettings), resolver)
}

func NewOCRProviderLister(repos *repository.RepoContainer, resolver *ocrcore.ConfigResolver) *ocrcore.ProviderLister {
	return ocrcore.NewProviderLister(ocrcore.NewSettingsAdapter(repos.OCRSettings), resolver)
}

func NewOCRRulesChecker(repos *repository.RepoContainer, resolver *ocrcore.ConfigResolver) *ocrcore.RulesChecker {
	return ocrcore.NewRulesChecker(ocrcore.NewSettingsAdapter(repos.OCRSettings), nil, resolver)
}

func NewOCRVersioner(repos *repository.RepoContainer, resolver *ocrcore.ConfigResolver) *ocrcore.Versioner {
	return ocrcore.NewVersioner(ocrcore.NewSettingsAdapter(repos.OCRSettings), resolver)
}

func NewOCRViewerManager(repos *repository.RepoContainer, resolver *ocrcore.ConfigResolver) *ocrcore.Viewer {
	return ocrcore.NewViewer(ocrcore.NewSettingsAdapter(repos.OCRSettings), resolver)
}

func NewOCRUpgrader(repos *repository.RepoContainer, resolver *ocrcore.ConfigResolver, versioner *ocrcore.Versioner, cfg *config.Config) *ocrcore.Upgrader {
	return ocrcore.NewUpgrader(
		ocrcore.NewSettingsAdapter(repos.OCRSettings),
		resolver,
		versioner,
		cfg.OCR.UpgradeInstallDir,
		cfg.OCR.GitHubRepo,
	)
}

// _ 避免 unused import
var _ = struct{}{}