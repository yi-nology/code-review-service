package settings_service

import (
	svcApp "github.com/yi-nology/code-review-service/internal/app"
	"github.com/yi-nology/code-review-service/internal/ocr"
	"github.com/yi-nology/code-review-service/internal/repository"
)

// god object 拆分：原 SettingsHandler（44 方法，archcheck 阈值 30）按 settings
// 域收敛为 9 个子 handler，每个只持自己的 deps（依赖面从全量 8 repo + gitMgr
// + ocr 缩到每域 1-2 个）。biz/router 的包级委托名保持不变。
// 各域类型/构造器定义在其所属域文件旁（见 var 块下方 new* 函数集中于此）。

// 各域子 handler 的包级实例（InitHandler 装配）。
var agentSettingsInst *AgentSettingsHandler
var customProviderInst *CustomProviderHandler
var ocrAssetsInst *OCRAssetsHandler
var ocrToolsInst *OCRToolsHandler
var globalRulesInst *GlobalRulesHandler
var ocrUpgradeInst *OCRUpgradeHandler
var projectPolicyInst *ProjectPolicyHandler
var ocrSettingsInst *OCRSettingsHandler
var platformInst *PlatformSettingsHandler

func newAgentSettingsHandler(repos *repository.RepoContainer) *AgentSettingsHandler {
	return &AgentSettingsHandler{
		agentRepo: repos.ReviewerAgent,
	}
}

func newCustomProviderHandler(repos *repository.RepoContainer) *CustomProviderHandler {
	return &CustomProviderHandler{
		customProvider: repos.CustomProvider,
	}
}

func newOCRAssetsHandler(repos *repository.RepoContainer) *OCRAssetsHandler {
	return &OCRAssetsHandler{
		rulePackages: repos.OCRRulePackage,
		toolPackages: repos.OCRToolPackage,
	}
}

func newOCRToolsHandler(repos *repository.RepoContainer, ocrReg *ocr.Registry) *OCRToolsHandler {
	return &OCRToolsHandler{
		ocr: ocrReg,
	}
}

func newGlobalRulesHandler(repos *repository.RepoContainer) *GlobalRulesHandler {
	return &GlobalRulesHandler{
		globalRules: repos.GlobalRules,
	}
}

func newOCRUpgradeHandler(repos *repository.RepoContainer, ocrReg *ocr.Registry) *OCRUpgradeHandler {
	return &OCRUpgradeHandler{
		ocr: ocrReg,
	}
}

func newProjectPolicyHandler(repos *repository.RepoContainer) *ProjectPolicyHandler {
	return &ProjectPolicyHandler{
		project: repos.ProjectPolicy,
	}
}

func newOCRSettingsHandler(repos *repository.RepoContainer, ocrReg *ocr.Registry) *OCRSettingsHandler {
	return &OCRSettingsHandler{
		ocrRepo: repos.OCRSettings,
		ocr:     ocrReg,
	}
}

func newPlatformSettingsHandler(repos *repository.RepoContainer, gitMgr svcApp.PlatformAdmin) *PlatformSettingsHandler {
	return &PlatformSettingsHandler{
		platform: repos.GitPlatform,
		gitMgr:   gitMgr,
	}
}

// 各域子 handler 类型定义（deps 按域最小化）。
type AgentSettingsHandler struct {
	agentRepo repository.ReviewerAgentRepository
}

type CustomProviderHandler struct {
	customProvider repository.CustomProviderRepository
}

type OCRAssetsHandler struct {
	rulePackages repository.OCRRulePackageRepository
	toolPackages repository.OCRToolPackageRepository
}

type OCRToolsHandler struct {
	ocr *ocr.Registry
}

type GlobalRulesHandler struct {
	globalRules repository.GlobalRulesRepository
}

type OCRUpgradeHandler struct {
	ocr *ocr.Registry
}

type ProjectPolicyHandler struct {
	project repository.ProjectPolicyRepository
}

type OCRSettingsHandler struct {
	ocrRepo repository.OCRSettingsRepository
	ocr     *ocr.Registry
}

type PlatformSettingsHandler struct {
	platform repository.GitPlatformRepository
	gitMgr   svcApp.PlatformAdmin
}

// InitHandler 收口为 2 个参数：repos（聚合 8 个 repo）+ ocr.Registry。
func InitHandler(
	repos *repository.RepoContainer,
	gitMgr svcApp.PlatformAdmin,
	ocrReg *ocr.Registry,
) {
	agentSettingsInst = newAgentSettingsHandler(repos)
	customProviderInst = newCustomProviderHandler(repos)
	ocrAssetsInst = newOCRAssetsHandler(repos)
	ocrToolsInst = newOCRToolsHandler(repos, ocrReg)
	globalRulesInst = newGlobalRulesHandler(repos)
	ocrUpgradeInst = newOCRUpgradeHandler(repos, ocrReg)
	projectPolicyInst = newProjectPolicyHandler(repos)
	ocrSettingsInst = newOCRSettingsHandler(repos, ocrReg)
	platformInst = newPlatformSettingsHandler(repos, gitMgr)
}
