package modules

import (
	"context"

	ocrcore "github.com/yi-nology/code-review-service/internal/ocr"
	"github.com/yi-nology/code-review-service/internal/repository"

	"github.com/oekit/logx"
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

	// 服务关闭时回收 viewer 子进程，避免孤儿进程残留。
	fx.Invoke(RegisterViewerShutdown),
	// 服务关闭时停止 Runner 后台刷新 goroutine。
	fx.Invoke(RegisterRunnerShutdown),
)

// NewOCRRegistry 构造 *ocrcore.Registry 单一入口。
func NewOCRRegistry(
	repos *repository.RepoContainer,
	llmProvider *ocrcore.MetricsProvider,
	ocrRunner *ocrcore.Runner,
	resolver *ocrcore.ConfigResolver,
	versioner *ocrcore.Versioner,
) *ocrcore.Registry {
	// 让 Runner 能在每次执行时从 DB 动态解析最新配置（binary/language/concurrency 等），
	// 而非使用启动时的固定 fallback config。
	ocrRunner.WithResolver(resolver)

	fallbackConfig := ocrcore.Defaults().ToConfig()
	return ocrcore.New(
		repos,
		fallbackConfig,
		resolver,
		NewOCRTester(repos, resolver),
		NewOCRProviderLister(repos, resolver),
		NewOCRRulesChecker(repos, resolver),
		versioner,
		NewOCRViewerManager(repos, resolver),
		NewOCRUpgrader(repos, resolver, versioner),
		NewOCRSessionLister(repos, resolver),
		llmProvider,
	).WithRunner(ocrRunner, ocrRunner, ocrRunner)
}

// NewConfigResolver 阶段 A15 part 6：fallback 改用 ocr.Fallback。
func NewConfigResolver(repos *repository.RepoContainer) *ocrcore.ConfigResolver {
	fallback := ocrcore.Fallback{
		Binary:          "ocr",
		Audience:        "agent",
		DefaultSeverity: "major",
	}
	return ocrcore.NewConfigResolver(
		ocrcore.NewSettingsAdapter(repos.OCRSettings),
		fallback,
		newToolPackageSettings(repos.OCRToolPackage),
	)
}

func NewOCRTester(repos *repository.RepoContainer, resolver *ocrcore.ConfigResolver) *ocrcore.Tester {
	return ocrcore.NewTester(ocrcore.NewSettingsAdapter(repos.OCRSettings), resolver)
}

func NewOCRProviderLister(repos *repository.RepoContainer, resolver *ocrcore.ConfigResolver) *ocrcore.ProviderLister {
	return ocrcore.NewProviderLister(ocrcore.NewSettingsAdapter(repos.OCRSettings), resolver)
}

func NewOCRRulesChecker(repos *repository.RepoContainer, resolver *ocrcore.ConfigResolver) *ocrcore.RulesChecker {
	checker := ocrcore.NewRulesChecker(ocrcore.NewSettingsAdapter(repos.OCRSettings), newGlobalRulesAdapter(repos.GlobalRules), resolver)
	if repos.OCRRulePackage != nil {
		checker = checker.WithRulePackages(newRulePackageSource(repos.OCRRulePackage))
	}
	return checker
}

// globalRulesAdapter 把 repository.GlobalRulesRepository 适配成 ocrcore.GlobalRules（rules-check 用）。
func newGlobalRulesAdapter(repo repository.GlobalRulesRepository) ocrcore.GlobalRules {
	if repo == nil {
		return nil
	}
	return &globalRulesAdapter{repo: repo}
}

type globalRulesAdapter struct {
	repo repository.GlobalRulesRepository
}

func (a *globalRulesAdapter) Get(ctx context.Context) (*struct {
	Rules []byte
}, error) {
	g, err := a.repo.Get(ctx)
	if err != nil || g == nil {
		return nil, err
	}
	return &struct{ Rules []byte }{Rules: g.Content}, nil
}

// rulePackageSource 把 repository.OCRRulePackageRepository 适配成 ocrcore.RulePackageSource。
// 只返回启用规则包的 Rules（按 glob 文件模式的 JSON），供 rules-check 匹配。
func newRulePackageSource(repo repository.OCRRulePackageRepository) ocrcore.RulePackageSource {
	return &rulePackageSource{repo: repo}
}

type rulePackageSource struct {
	repo repository.OCRRulePackageRepository
}

func (s *rulePackageSource) ListEnabledRules(ctx context.Context) ([][]byte, error) {
	packages, err := s.repo.List(ctx)
	if err != nil {
		return nil, err
	}
	var result [][]byte
	for i := range packages {
		if !packages[i].Enabled {
			continue
		}
		result = append(result, packages[i].Rules)
	}
	return result, nil
}

// newToolPackageSettings 把 repository.OCRToolPackageRepository 适配成 ocrcore.ToolSettings。
// 补齐 ConfigResolver 的第三个配置来源：DB 里启用的工具包配置（JSON）
// 会在 BuildConfig 缺省 ToolsConfig 时生效（defaultToolsConfig 按 ScanType 匹配）。
// repo 为 nil 时返回 nil（ToolSettings 接口自身容忍 nil）。
func newToolPackageSettings(repo repository.OCRToolPackageRepository) ocrcore.ToolSettings {
	if repo == nil {
		return nil
	}
	return &toolPackageSettings{repo: repo}
}

type toolPackageSettings struct {
	repo repository.OCRToolPackageRepository
}

func (s *toolPackageSettings) List(ctx context.Context) ([]ocrcore.ToolPackageData, error) {
	packages, err := s.repo.List(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]ocrcore.ToolPackageData, 0, len(packages))
	for _, p := range packages {
		out = append(out, ocrcore.ToolPackageData{
			Name:     p.Name,
			ScanType: p.ScanType,
			Enabled:  p.Enabled,
			Config:   string(p.Config),
		})
	}
	return out, nil
}

func NewOCRVersioner(repos *repository.RepoContainer, resolver *ocrcore.ConfigResolver) *ocrcore.Versioner {
	return ocrcore.NewVersioner(ocrcore.NewSettingsAdapter(repos.OCRSettings), resolver)
}

func NewOCRViewerManager(repos *repository.RepoContainer, resolver *ocrcore.ConfigResolver) *ocrcore.Viewer {
	return ocrcore.NewViewer(ocrcore.NewSettingsAdapter(repos.OCRSettings), resolver)
}

func NewOCRUpgrader(repos *repository.RepoContainer, resolver *ocrcore.ConfigResolver, versioner *ocrcore.Versioner) *ocrcore.Upgrader {
	return ocrcore.NewUpgrader(
		ocrcore.NewSettingsAdapter(repos.OCRSettings),
		resolver,
		versioner,
		"", // UpgradeInstallDir: 从 DB 或默认值获取
		"", // GitHubRepo: 从 DB 或默认值获取
	)
}

func NewOCRSessionLister(repos *repository.RepoContainer, resolver *ocrcore.ConfigResolver) *ocrcore.SessionLister {
	return ocrcore.NewSessionLister(ocrcore.NewSettingsAdapter(repos.OCRSettings), resolver)
}

// RegisterViewerShutdown 在服务关闭时停止 viewer 子进程，避免孤儿进程残留。
func RegisterViewerShutdown(lc fx.Lifecycle, viewer *ocrcore.Viewer) {
	lc.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			status, err := viewer.StopViewer(ctx)
			if err != nil {
				logx.Warn("failed to stop ocr viewer on shutdown", "error", err)
				return nil
			}
			if status != nil && !status.Running {
				logx.Info("ocr viewer stopped on shutdown")
			}
			return nil
		},
	})
}

// RegisterRunnerShutdown 在服务关闭时停止 Runner 的后台配置刷新 goroutine。
func RegisterRunnerShutdown(lc fx.Lifecycle, registry *ocrcore.Registry) {
	lc.Append(fx.Hook{
		OnStop: func(_ context.Context) error {
			registry.StopRunner()
			return nil
		},
	})
}

// _ 避免 unused import
var _ = struct{}{}
