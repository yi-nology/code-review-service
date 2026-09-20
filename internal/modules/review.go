package modules

import (
	"context"

	"github.com/yi-nology/code-review-service/internal/argusclient"
	"github.com/yi-nology/code-review-service/internal/config"
	"github.com/yi-nology/code-review-service/internal/eino"
	"github.com/yi-nology/code-review-service/internal/ocr"
	"github.com/yi-nology/code-review-service/internal/repository"
	"github.com/yi-nology/code-review-service/internal/service/agent"
	"github.com/yi-nology/code-review-service/internal/service/cost"
	"github.com/yi-nology/code-review-service/internal/service/judge"
	"github.com/yi-nology/code-review-service/internal/service/policy"
	"github.com/yi-nology/code-review-service/internal/service/progress"
	gitProvider "github.com/yi-nology/code-review-service/internal/service/provider"
	svcReview "github.com/yi-nology/code-review-service/internal/service/review"
	"github.com/yi-nology/code-review-service/internal/service/review/domain"
	"github.com/yi-nology/code-review-service/internal/service/reviewenrich"
	"github.com/yi-nology/code-review-service/internal/service/statuscheck"
	"github.com/yi-nology/code-review-service/internal/service/workflow"
	"github.com/yi-nology/code-review-service/internal/service/writeback"

	"go.uber.org/fx"
)

var ReviewModule = fx.Options(
	fx.Provide(NewProgressManager),
	fx.Provide(NewPolicyEngine),
	fx.Provide(NewJudge),
	fx.Provide(NewFailureReporter),
	fx.Provide(reviewenrich.New),
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

// NewPolicyEngine 构造门禁引擎：启动时以 DB 当前值兜底，并挂执行期动态源——
// 每次评审评估门禁时重新读 OCRSettings（web console 改 policy 配置无需重启生效，
// 与「业务配置存 DB 热更新」原则一致）。读取失败回落启动配置，门禁始终可用。
func NewPolicyEngine(repos *repository.RepoContainer) *policy.Engine {
	buildCfg := func() policy.Config {
		cfg := policy.DefaultConfig()
		if repos != nil && repos.OCRSettings != nil {
			if s, err := repos.OCRSettings.Get(context.Background()); err == nil && s != nil {
				if s.PolicyFailOn != "" {
					cfg.FailOn = s.PolicyFailOn
				}
				if s.PolicyMaxComments > 0 {
					cfg.MaxComments = s.PolicyMaxComments
				}
				cfg.SummaryOnly = s.PolicySummaryOnly
				if len(s.PolicyIgnorePaths) > 0 {
					cfg.IgnorePaths = s.PolicyIgnorePaths
				}
			}
		}
		return cfg
	}

	return policy.NewEngine(buildCfg()).WithSource(buildCfg)
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
	FailureReporter *svcReview.FailureReporter
	GitProviderSvc  *gitProvider.GitProviderService
	Config          *config.Config
	BudgetChecker   *cost.BudgetChecker
	CostCalc        *cost.Calculator
	Judge           *judge.Judge // OCR 结果二次综合判断
}

// NewJudge 构造 OCR 结果二次综合判断器。
// 使用 ocr.Provider（LLM）对 OCR 产出的 findings 做二次分析。
func NewJudge(llmProvider ocr.Provider) *judge.Judge {
	return judge.NewJudge(llmProvider)
}

// NewFailureReporter 构造统一失败上报器（eino 执行器与 review worker 共用同一实例，
// 保证执行前失败与执行中失败的副作用集一致：历史/落库/通知/commit status/指标）。
func NewFailureReporter(repos *repository.RepoContainer, notifier domain.Notifier,
	gitSvc *gitProvider.GitProviderService, cfg *config.Config) *svcReview.FailureReporter {
	return svcReview.NewFailureReporter(repos.Review, notifier,
		statuscheck.NewPoster(gitSvc, cfg.ExternalBaseURL))
}

func NewExecutorService(deps ExecutorDeps) *eino.Executor {
	return eino.NewExecutor(eino.ExecutorConfig{
		ReviewRepo:      deps.Repos.Review,
		ProjectRepo:     deps.Repos.ProjectPolicy,
		GlobalRulesRepo: deps.Repos.GlobalRules,
		RulePackageRepo: deps.Repos.OCRRulePackage,
		AgentRepo:       deps.Repos.ReviewerAgent,
		OCRSettingsRepo: deps.Repos.OCRSettings,
		Runner:          deps.OcrRunner,
		Writeback:       deps.WritebackPub,
		Notifier:        deps.Notifier,
		StatusPoster:    statuscheck.NewPoster(deps.GitProviderSvc, deps.Config.ExternalBaseURL),
		FailureReporter: deps.FailureReporter,
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
		Judge:        deps.Judge,
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
	ExecutorSvc     *eino.Executor
	FailureReporter *svcReview.FailureReporter
	ArgusClient     *argusclient.Client
}

func NewReviewService(deps ReviewServiceDeps) *svcReview.Service {
	// 引擎分流：engine=ocr（默认）走原 OCR 执行器（一行不改）；
	// engine=argus 走 ArgusExecutor（触发外部任务，push 回报收敛终态）。
	// argus 未配置时创建期即拒绝 engine=argus（ArgusEnabled=false）。
	// statusPoster 让 argus 路径与 OCR 一样在 PR 上留门禁 status（pending→终态）。
	argusExecutor := svcReview.NewArgusExecutor(deps.ArgusClient, deps.Repos.Review,
		statuscheck.NewPoster(deps.GitProviderSvc, deps.Config.ExternalBaseURL))
	return svcReview.NewService(svcReview.ServiceConfig{
		ReviewRepo:      deps.Repos.Review,
		OCRSettingsRepo: deps.Repos.OCRSettings,
		ProjectRepo:     deps.Repos.ProjectPolicy,
		GlobalRulesRepo: deps.Repos.GlobalRules,
		RulePackageRepo: deps.Repos.OCRRulePackage,
		AgentRepo:       deps.Repos.ReviewerAgent,
		SessionRepo:     deps.SessionRepo,
		Policy:          deps.PolicyEngine,
		Providers:       deps.GitProviders,
		Runner:          deps.OcrRunner,
		PreviewRunner:   deps.OcrRunner,
		ScanRunner:      deps.OcrRunner,
		Delegator:       deps.OcrRunner, // *ocr.Runner 同时实现 domain.Delegator（OCR v1.7.11+）
		ProgressManager: deps.ProgressManager,
		Executor:        svcReview.NewEngineExecutor(deps.ExecutorSvc, argusExecutor),
		FailureReporter: deps.FailureReporter,
		Concurrency:     deps.Config.Review.WorkerConcurrency,
		ArgusEnabled:    deps.ArgusClient.Enabled(),
	})
}
