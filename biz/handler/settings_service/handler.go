package settings_service

import (
	svcApp "github.com/yi-nology/code-review-service/internal/app"
	"github.com/yi-nology/code-review-service/internal/ocr"
	"github.com/yi-nology/code-review-service/internal/repository"
)

var handlerInst *SettingsHandler

// SettingsHandler 收口为 1 个 *ocr.Registry 字段。
// B 阶段：保留 8 个 repository 字段 + 1 个 gitMgr + 1 个 ocr 字段。
// A 阶段：repository 字段可下沉到 ocr.Registry.Repos()。
type SettingsHandler struct {
	ocrRepo        repository.OCRSettingsRepository
	project        repository.ProjectPolicyRepository
	platform       repository.GitPlatformRepository
	globalRules    repository.GlobalRulesRepository
	rulePackages   repository.OCRRulePackageRepository
	toolPackages   repository.OCRToolPackageRepository
	agentRepo      repository.ReviewerAgentRepository
	customProvider repository.CustomProviderRepository
	gitMgr         svcApp.GitProviderManager
	// ocr 聚合了 tester/lister/checker/versioner/viewer/upgrader 6 个能力。
	ocr *ocr.Registry
}

// InitHandler 收口为 2 个参数：repos（聚合 8 个 repo）+ ocr.Registry。
// 之前 14 个参数；B 阶段拆为 2 个，未来可进一步压成 1 个 *RepoContainer。
func InitHandler(
	repos *repository.RepoContainer,
	gitMgr svcApp.GitProviderManager,
	ocrReg *ocr.Registry,
) {
	handlerInst = &SettingsHandler{
		ocrRepo:        repos.OCRSettings,
		project:        repos.ProjectPolicy,
		platform:       repos.GitPlatform,
		globalRules:    repos.GlobalRules,
		rulePackages:   repos.OCRRulePackage,
		toolPackages:   repos.OCRToolPackage,
		agentRepo:      repos.ReviewerAgent,
		customProvider: repos.CustomProvider,
		gitMgr:         gitMgr,
		ocr:            ocrReg,
	}
}
