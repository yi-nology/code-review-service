package modules

import (
	"context"

	"github.com/yi-nology/code-review-service/internal/argusclient"
	"github.com/yi-nology/code-review-service/internal/config"
	"github.com/yi-nology/code-review-service/internal/domain/review"
	"github.com/yi-nology/code-review-service/internal/eino"
	"github.com/yi-nology/code-review-service/internal/ocr"
	"github.com/yi-nology/code-review-service/internal/repository"
	"github.com/yi-nology/code-review-service/internal/service/agent"
	"github.com/yi-nology/code-review-service/internal/service/cost"
	"github.com/yi-nology/code-review-service/internal/service/judge"
	"github.com/yi-nology/code-review-service/internal/service/memory"
	"github.com/yi-nology/code-review-service/internal/service/policy"
	"github.com/yi-nology/code-review-service/internal/service/progress"
	gitProvider "github.com/yi-nology/code-review-service/internal/service/provider"
	svcReview "github.com/yi-nology/code-review-service/internal/service/review"
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
	fx.Provide(NewReviewPipeline),
	fx.Provide(NewReviewStateStore),
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
	Judge           *judge.Judge    // OCR 结果二次综合判断
	Memory          *memory.Service // 跨会话长期记忆（nil = 未启用）
	// Pipeline 评审前置编排（窄端口；实现在 service/review）。
	Pipeline svcReview.PipelineRunner
	// Store 评审状态存储（窄接口）。
	Store svcReview.ReviewStateStore
}

// NewJudge 构造 OCR 结果二次综合判断器。
// 使用 ocr.Provider（LLM）对 OCR 产出的 findings 做二次分析。
func NewJudge(llmProvider ocr.Provider) *judge.Judge {
	return judge.NewJudge(llmProvider)
}

// NewFailureReporter 构造统一失败上报器（eino 执行器与 review worker 共用同一实例，
// 保证执行前失败与执行中失败的副作用集一致：历史/落库/通知/commit status/PR 评论/指标）。
func NewFailureReporter(repos *repository.RepoContainer, notifier domain.Notifier,
	gitSvc *gitProvider.GitProviderService, writebackPub *writeback.WritebackPublisherAdapter,
	cfg *config.Config) *svcReview.FailureReporter {
	return svcReview.NewFailureReporter(repos.Review, notifier,
		statuscheck.NewPoster(gitSvc, cfg.ExternalBaseURL)).WithCommenter(writebackPub)
}

// NewReviewPipeline 构造评审前置编排器（克隆工作区 → 项目策略 → 文件过滤 →
// 规则合并 → 记忆召回 → OCR 执行参数）。
//
// 它在**装配层**构造：仓储、Agent 名册、记忆服务在这里汇入，引擎侧只拿到
// 窄端口（svcReview.PipelineRunner）——引擎适配层因此不再直接依赖仓储包，
// 依赖方向不再倒置（见 internal/eino/executor.go 的说明）。
func NewReviewPipeline(
	repos *repository.RepoContainer,
	agentMgr *agent.AgentManager,
	mem *memory.Service,
) svcReview.PipelineRunner {
	// 记忆未启用时 mem 是 nil 指针：塞进接口会变成「非 nil 接口持有 nil 指针」，
	// 调用即 panic——所以这里显式留 nil 接口。
	var recaller svcReview.MemoryRecaller
	if mem != nil {
		recaller = mem
	}
	var registry svcReview.AgentRegistry
	if agentMgr != nil {
		registry = agentMgr
	}
	return svcReview.NewPipeline(
		repos.Review,
		repos.ProjectPolicy,
		repos.GlobalRules,
		repos.OCRRulePackage,
		repos.ReviewerAgent,
		repos.OCRSettings,
		registry,
		recaller,
	)
}

// NewReviewStateStore 提供评审状态的窄存储接口。
//
// 引擎侧只认这个窄接口，不认完整的 repository.ReviewRepository——
// 这样引擎适配层就不必 import 仓储包。
func NewReviewStateStore(repos *repository.RepoContainer) svcReview.ReviewStateStore {
	return repos.Review
}

func NewExecutorService(deps ExecutorDeps) *eino.Executor {
	return eino.NewExecutor(eino.ExecutorConfig{
		Pipeline:        deps.Pipeline,
		Store:           deps.Store,
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
		Memory:       deps.Memory,
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
		ReviewRepo:        deps.Repos.Review,
		OCRSettingsRepo:   deps.Repos.OCRSettings,
		ProjectRepo:       deps.Repos.ProjectPolicy,
		GlobalRulesRepo:   deps.Repos.GlobalRules,
		RulePackageRepo:   deps.Repos.OCRRulePackage,
		AgentRepo:         deps.Repos.ReviewerAgent,
		SessionRepo:       deps.SessionRepo,
		Policy:            deps.PolicyEngine,
		Providers:         deps.GitProviders,
		Runner:            deps.OcrRunner,
		PreviewRunner:     deps.OcrRunner,
		ScanRunner:        deps.OcrRunner,
		Delegator:         deps.OcrRunner, // *ocr.Runner 同时实现 domain.Delegator（OCR v1.7.11+）
		ProgressManager:   deps.ProgressManager,
		Executor:          svcReview.NewEngineExecutor(deps.ExecutorSvc, argusExecutor),
		FailureReporter:   deps.FailureReporter,
		Concurrency:       deps.Config.Review.WorkerConcurrency,
		HeartbeatInterval: deps.Config.GetWorkerStaleAfter() / 3,
		ArgusEnabled:      deps.ArgusClient.Enabled(),
	})
}
