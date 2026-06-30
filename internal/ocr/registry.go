// Package ocr 是「open-code-review 能力」的注册中心。
//
// 背景：原 internal/service/ocr + internal/service/runner + internal/service/llm
// 三个包各自实现 open-code-review 的某个子能力（tester / versioner / upgrader /
// viewer / lister / checker / LLM provider / runner），导致：
//   - 7 个 manager 各自重复读 settings / 解析 binary / 调 exec
//   - 5 个等价 settingsSource 接口分散在 3 个包
//   - handler 端 InitHandler 参数列表 14 个，模块侧 fx.Provide 声明 12 个
//
// Registry 把所有能力收口为一个对象，handler / 模块通过 Registry 一次性拿到。
//
// 阶段 B（当前 PR）：Registry 是「薄壳」，实现仍由 runner/ocr/llm 三个老包提供。
// 阶段 A（OCR_CORE_DESIGN.md 后续 PR）：registry 升级为 full Client，把三个老包吸收到
// 单一的 internal/ocr 核心包。
package ocr

import (
	"github.com/yi-nology/code-review-service/internal/repository"
	"github.com/yi-nology/code-review-service/internal/service/llm"
	"github.com/yi-nology/code-review-service/internal/service/review/domain"
	"github.com/yi-nology/code-review-service/internal/service/runner"
)

// Registry 是 open-code-review 能力的注册中心，handler/模块一次性拿到所有子能力。
//
// 设计原则：
//   - Registry 不持有任何状态，所有能力都是 getter 转发到底层 manager
//   - 构造完成后不可变；如需重载配置，调用 Refresh()（阶段 A 引入）
//   - 零依赖：registry 本身只 import domain + runner/ocr/llm + repository，不引入新包
type Registry struct {
	repos *repository.RepoContainer

	runner    domain.Runner
	previewer domain.PreviewRunner
	scanner   domain.ScanRunner
	tester    domain.LLMTester
	lister    domain.LLMProviderLister
	checker   domain.RulesChecker
	versioner domain.OCRVersioner
	viewer    domain.ViewerManager
	upgrader  domain.OCRUpgrader

	llmProvider llm.Provider
}

// New 从已构造的子 manager 构造 Registry。
//
// 仍保留 ocr.NewTester / ocr.NewVersioner 等老构造函数供模块按需 fx.Provide，
// 但强烈推荐新代码直接拿 *Registry。
//
// 注：resolver 暂未注入到 Registry（OCRBinaryCheck 仍通过 repos + 旧 resolver 拿到）。
// 阶段 A 统一迁入后此参数消失。
func New(
	repos *repository.RepoContainer,
	fallback *runner.OpenCodeReviewRunner,
	tester domain.LLMTester,
	lister domain.LLMProviderLister,
	checker domain.RulesChecker,
	versioner domain.OCRVersioner,
	viewer domain.ViewerManager,
	upgrader domain.OCRUpgrader,
	llmProvider llm.Provider,
) *Registry {
	// runner.NewDynamicOpenCodeReviewRunner 同时实现 domain.Runner / PreviewRunner / ScanRunner
	// 由调用方预构造后注入；这里仅做 getter 转发。
	_ = fallback
	return &Registry{
		repos:       repos,
		runner:      nil, // 由模块侧把 DynamicOpenCodeReviewRunner 同时赋给三个 getter
		previewer:   nil,
		scanner:     nil,
		tester:      tester,
		lister:      lister,
		checker:     checker,
		versioner:   versioner,
		viewer:      viewer,
		upgrader:    upgrader,
		llmProvider: llmProvider,
	}
}

// WithRunner 显式设置 runner/previewer/scanner 三个 getter 共用的实例。
// 由于 domain.Runner / PreviewRunner / ScanRunner 三个接口通常由同一个
// *DynamicOpenCodeReviewRunner 实现，调用方应传入同一实例以避免重复。
func (r *Registry) WithRunner(rr domain.Runner, pp domain.PreviewRunner, ss domain.ScanRunner) *Registry {
	r.runner = rr
	r.previewer = pp
	r.scanner = ss
	return r
}

// -------- 能力 getter --------

// Runner 返回评审执行器。
func (r *Registry) Runner() domain.Runner { return r.runner }

// Previewer 返回预览器。
func (r *Registry) Previewer() domain.PreviewRunner { return r.previewer }

// Scanner 返回扫描器。
func (r *Registry) Scanner() domain.ScanRunner { return r.scanner }

// Tester 返回 LLM 连通性测试器。
func (r *Registry) Tester() domain.LLMTester { return r.tester }

// Lister 返回 LLM provider 列表器。
func (r *Registry) Lister() domain.LLMProviderLister { return r.lister }

// Checker 返回规则检查器。
func (r *Registry) Checker() domain.RulesChecker { return r.checker }

// Versioner 返回 OCR 版本读取器。
func (r *Registry) Versioner() domain.OCRVersioner { return r.versioner }

// Viewer 返回 OCR viewer 管理器。
func (r *Registry) Viewer() domain.ViewerManager { return r.viewer }

// Upgrader 返回 OCR 在线升级器。
func (r *Registry) Upgrader() domain.OCRUpgrader { return r.upgrader }

// LLM 返回统一 LLM Provider，供 discussion / tech-lead 等阶段直接调用。
func (r *Registry) LLM() llm.Provider { return r.llmProvider }

// Settings 暴露 OCRSettingsRepository，方便 readiness / health 调用。
func (r *Registry) Settings() repository.OCRSettingsRepository { return r.repos.OCRSettings }

// ToolPackages 同上。
func (r *Registry) ToolPackages() repository.OCRToolPackageRepository {
	return r.repos.OCRToolPackage
}

// RulePackages 同上。
func (r *Registry) RulePackages() repository.OCRRulePackageRepository {
	return r.repos.OCRRulePackage
}

// GlobalRules 同上。
func (r *Registry) GlobalRules() repository.GlobalRulesRepository {
	return r.repos.GlobalRules
}

// Repos 返回底层 RepoContainer 供特殊场景使用（如 readiness 检查）。
func (r *Registry) Repos() *repository.RepoContainer { return r.repos }
