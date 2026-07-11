package modules

import (
	"context"

	"github.com/yi-nology/code-review-service/internal/config"
	"github.com/yi-nology/code-review-service/internal/ocr"
	"github.com/yi-nology/code-review-service/internal/repository"
	"github.com/yi-nology/code-review-service/internal/service/agent"
	"github.com/yi-nology/code-review-service/internal/service/cost"
	"github.com/yi-nology/code-review-service/internal/service/executor"
	"github.com/yi-nology/code-review-service/internal/service/policy"
	"github.com/yi-nology/code-review-service/internal/service/progress"
	gitProvider "github.com/yi-nology/code-review-service/internal/service/provider"
	svcReview "github.com/yi-nology/code-review-service/internal/service/review"
	"github.com/yi-nology/code-review-service/internal/service/review/domain"
	"github.com/yi-nology/code-review-service/internal/service/statuscheck"
	"github.com/yi-nology/code-review-service/internal/service/workflow"
	"github.com/yi-nology/code-review-service/internal/service/writeback"

	"go.uber.org/fx"
)

var ReviewModule = fx.Options(
	fx.Provide(NewProgressManager),
	fx.Provide(NewPolicyEngine),
	fx.Provide(NewExecutorService),
	fx.Provide(NewReviewService),
)

func NewProgressManager(lc fx.Lifecycle) *progress.ProgressManager {
	pm := progress.NewProgressManager()
	lc.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			pm.Close()
			return nil
		},
	})
	return pm
}

func NewPolicyEngine(cfg *config.Config) *policy.Engine {
	return policy.NewEngine(policy.Config{
		FailOn:      cfg.Policy.FailOn,
		MaxComments: cfg.Policy.MaxComments,
		SummaryOnly: cfg.Policy.SummaryOnly,
		IgnorePaths: cfg.Policy.IgnorePaths,
	})
}

type ExecutorDeps struct {
	fx.In

	Repos           *repository.RepoContainer
	OcrRunner       *ocr.Runner
	WritebackOrch   *writeback.Orchestrator
	WritebackPub    *writeback.WritebackPublisherAdapter
	PolicyEngine    *policy.Engine
	ProgressManager *progress.ProgressManager
	Workflow        *workflow.ReviewWorkflow
	AgentManager    *agent.AgentManager
	LLMMetrics      *ocr.Metrics
	Notifier        domain.Notifier
	GitProviderSvc  *gitProvider.GitProviderService
	BudgetChecker   *cost.BudgetChecker
	CostCalc        *cost.Calculator
}

func NewExecutorService(deps ExecutorDeps) *executor.Service {
	return executor.NewService(executor.Config{
		ReviewRepo:      deps.Repos.Review,
		ProjectRepo:     deps.Repos.ProjectPolicy,
		GlobalRulesRepo: deps.Repos.GlobalRules,
		RulePackageRepo: deps.Repos.OCRRulePackage,
		Runner:          deps.OcrRunner,
		Writeback:       deps.WritebackPub,
		Notifier:        deps.Notifier,
		StatusPoster:    statuscheck.NewPoster(deps.GitProviderSvc),
		BudgetChecker:   deps.BudgetChecker,
		CostCalc:        deps.CostCalc,
		ModelNameFunc: func() string {
			// 延迟读取当前 OCR 设置的模型名（评审完成时拿最新配置）。
			if s, err := deps.Repos.OCRSettings.Get(context.Background()); err == nil && s != nil {
				return s.ModelName
			}
			return ""
		},
		Policy:       deps.PolicyEngine,
		ProgressMgr:  deps.ProgressManager,
		Workflow:     deps.Workflow,
		AgentManager: deps.AgentManager,
		LLMMetrics:   deps.LLMMetrics,
	})
}

type ReviewServiceDeps struct {
	fx.In

	Config          *config.Config
	Repos           *repository.RepoContainer
	SessionRepo     repository.SessionRepository
	GitProviderSvc  *gitProvider.GitProviderService
	GitProviders    *gitProvider.GitProviderProviders
	OcrRunner       *ocr.Runner
	ProgressManager *progress.ProgressManager
	PolicyEngine    *policy.Engine
	ExecutorSvc     *executor.Service
}

func NewReviewService(deps ReviewServiceDeps) *svcReview.Service {
	return svcReview.NewService(svcReview.ServiceConfig{
		ReviewRepo:         deps.Repos.Review,
		OCRSettingsRepo:    deps.Repos.OCRSettings,
		ProjectRepo:        deps.Repos.ProjectPolicy,
		GlobalRulesRepo:    deps.Repos.GlobalRules,
		RulePackageRepo:    deps.Repos.OCRRulePackage,
		CustomProviderRepo: deps.Repos.CustomProvider,
		SessionRepo:        deps.SessionRepo,
		GitProviderSvc:  deps.GitProviderSvc,
		Policy:          deps.PolicyEngine,
		Providers:       deps.GitProviders,
		Runner:          deps.OcrRunner,
		PreviewRunner:   deps.OcrRunner,
		ScanRunner:      deps.OcrRunner,
		ProgressManager: deps.ProgressManager,
		Executor:        deps.ExecutorSvc,
		Concurrency:     deps.Config.Review.WorkerConcurrency,
	})
}
