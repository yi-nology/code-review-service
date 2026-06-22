package settings_service

import (
	svcApp "github.com/yi-nology/code-review-service/internal/app"
	"github.com/yi-nology/code-review-service/internal/repository"
	"github.com/yi-nology/code-review-service/internal/service/review"
)

var handlerInst *SettingsHandler

type SettingsHandler struct {
	ocrRepo        repository.OCRSettingsRepository
	project        repository.ProjectPolicyRepository
	platform       repository.GitPlatformRepository
	globalRules    repository.GlobalRulesRepository
	agentRepo      repository.ReviewerAgentRepository
	customProvider repository.CustomProviderRepository
	gitMgr         svcApp.GitProviderManager
	llmTester      review.LLMTester
	providerLister review.LLMProviderLister
	rulesChecker   review.RulesChecker
	ocrVersioner   review.OCRVersioner
	viewerMgr      review.ViewerManager
}

func InitHandler(
	ocrRepo repository.OCRSettingsRepository,
	project repository.ProjectPolicyRepository,
	platform repository.GitPlatformRepository,
	globalRules repository.GlobalRulesRepository,
	agentRepo repository.ReviewerAgentRepository,
	customProvider repository.CustomProviderRepository,
	gitMgr svcApp.GitProviderManager,
	llmTester review.LLMTester,
	providerLister review.LLMProviderLister,
	rulesChecker review.RulesChecker,
	ocrVersioner review.OCRVersioner,
	viewerMgr review.ViewerManager,
) {
	handlerInst = &SettingsHandler{
		ocrRepo:        ocrRepo,
		project:        project,
		platform:       platform,
		globalRules:    globalRules,
		agentRepo:      agentRepo,
		customProvider: customProvider,
		gitMgr:         gitMgr,
		llmTester:      llmTester,
		providerLister: providerLister,
		rulesChecker:   rulesChecker,
		ocrVersioner:   ocrVersioner,
		viewerMgr:      viewerMgr,
	}
}
