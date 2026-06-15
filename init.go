package main

// init.go - 应用初始化（手工维护）
//
// 代码组织说明:
//
// ┌─────────────────────────────────────────────────────────────────────┐
// │ IDL 维护 (hz 生成)                                                  │
// │   idl/*.thrift          → Thrift IDL 定义 (service + type)          │
// │   biz/model/            → 模型代码 (hz 从 IDL 类型生成)              │
// │   biz/router/           → 路由注册 (hz 从 IDL service 生成)          │
// │   biz/router/register.go → 主路由注册 (手工扩展 handler 初始化)      │
// │   biz/handler/*_service/ → 路由桩 (hz 生成, 委托给 handler 实例)     │
// │   swagger/openapi.yaml  → OpenAPI spec (thrift-gen-http-swagger 生成)│
// │   swagger/swagger.go    → Swagger UI 服务代码                       │
// ├─────────────────────────────────────────────────────────────────────┤
// │ 手工维护                                                            │
// │   biz/handler/common/   → 共享工具函数 (Fail, PageParams 等)        │
// │   biz/handler/review_service/    → ReviewHandler + WebhookHandler   │
// │   biz/handler/settings_service/  → SettingsHandler                  │
// │   biz/handler/stats_service/     → StatsHandler                     │
// │   biz/handler/session_service/   → SessionHandler                   │
// │   biz/handler/progress_service/  → ProgressWSHandler                │
// │   biz/handler/review_map_service/→ ReviewMapHandler                 │
// │   biz/handler/publisher_service/ → PublisherHandler                  │
// │   biz/handler/project_agent_service/ → ProjectAgentHandler          │
// │   internal/             → 业务逻辑层 (service, repository 等)       │
// │   init.go               → 应用初始化 (本文件)                       │
// │   server.go             → 服务器配置 + 路由注册入口                  │
// └─────────────────────────────────────────────────────────────────────┘
//
// 流程: init.go 创建 handler 实例 → server.go 调用 router.GeneratedRegister
//       → register.go 初始化 handler 单例 → 各 sub-router.Register 注册路由
//       → 路由桩函数委托给 handler 实例方法

import (
	"context"
	"fmt"
	"time"

	"github.com/yi-nology/code-review-service/biz/handler/progress_service"
	"github.com/yi-nology/code-review-service/biz/handler/project_agent_service"
	"github.com/yi-nology/code-review-service/biz/handler/publisher_service"
	"github.com/yi-nology/code-review-service/biz/handler/review_map_service"
	"github.com/yi-nology/code-review-service/biz/handler/review_service"
	"github.com/yi-nology/code-review-service/biz/handler/session_service"
	"github.com/yi-nology/code-review-service/biz/handler/settings_service"
	"github.com/yi-nology/code-review-service/biz/handler/stats_service"
	"github.com/yi-nology/code-review-service/internal/config"
	"github.com/yi-nology/code-review-service/internal/repository"
	"github.com/yi-nology/code-review-service/internal/repository/postgres"
	"github.com/yi-nology/code-review-service/internal/repository/postgres/models"
	"github.com/yi-nology/code-review-service/internal/service/agent"
	"github.com/yi-nology/code-review-service/internal/service/policy"
	"github.com/yi-nology/code-review-service/internal/service/progress"
	gitProvider "github.com/yi-nology/code-review-service/internal/service/provider"
	"github.com/yi-nology/code-review-service/internal/service/publisher"
	"github.com/yi-nology/code-review-service/internal/service/runner"
	svcReview "github.com/yi-nology/code-review-service/internal/service/review"
	"github.com/yi-nology/code-review-service/internal/service/session"
	statsSvc "github.com/yi-nology/code-review-service/internal/service/stats"
	"github.com/yi-nology/code-review-service/internal/service/webhook"
	"github.com/yi-nology/code-review-service/internal/service/writeback"

	"github.com/oekit/bootstrap"
	"github.com/cloudwego/hertz/pkg/common/hlog"
	gitplatform "github.com/yi-nology/git-platform-sdk/provider"
)

func runApp(cfg *config.Config, app *bootstrap.App) error {
	ctx := app.Context()

	db, err := postgres.InitDB(cfg.GetDatabaseConfig())
	if err != nil {
		hlog.Fatalf("init db failed: %v", err)
	}
	app.AddResource("db", func(ctx context.Context) error {
		return postgres.CloseDB(db)
	})

	repos := repository.NewRepoContainer(db)

	gitProviderSvc, err := initGitProviders(ctx, cfg, repos)
	if err != nil {
		return fmt.Errorf("init git providers: %w", err)
	}
	hlog.Infof("git providers initialized: %v", gitProviderSvc.GetSupportedPlatforms())

	ocrRunner := runner.NewDynamicOpenCodeReviewRunner(
		runner.NewOpenCodeReviewRunner(runner.OpenCodeReviewConfig{
			Binary:          cfg.OCR.Binary,
			RulePath:        cfg.OCR.Rule,
			DefaultSeverity: "major",
		}),
		repos.OCRSettings,
	)

	pub := publisher.New(publisher.Config{
		Enabled:     cfg.Policy.MaxComments > 0,
		MaxComments: cfg.Policy.MaxComments,
		SummaryOnly: cfg.Policy.SummaryOnly,
	}, gitProviderSvc)

	writebackOrchestrator := writeback.NewOrchestrator(writeback.OrchestratorConfig{
		Adapters: map[string]writeback.Adapter{
			"github": writeback.NewGitHubAdapter(gitProviderSvc),
			"gitlab": writeback.NewGitLabAdapter(gitProviderSvc),
			"gitea":  writeback.NewGiteaAdapter(gitProviderSvc),
		},
		ItemRepo:    repos.WritebackItem,
		ChangeRepo:  repos.ChangeRevision,
		MaxComments: cfg.Policy.MaxComments,
		SummaryOnly: cfg.Policy.SummaryOnly,
	})

	// Initialize progress manager (used for OCR progress tracking)
	progressManager := progress.NewProgressManager()

	reviewService := svcReview.NewService(svcReview.ServiceConfig{
		ReviewRepo:      repos.Review,
		OCRSettingsRepo: repos.OCRSettings,
		ProjectRepo:     repos.ProjectPolicy,
		GlobalRulesRepo: repos.GlobalRules,
		ReviewMapRepo:   repos.ReviewMap,
		Policy:          policy.NewEngine(cfg.GetPolicyConfig()),
		Providers:       gitProvider.NewGitProviderProviders(gitProviderSvc, int64(cfg.Review.MaxDiffBytes)),
		GitProvider:     gitProvider.NewGitProviderAdapter(gitProviderSvc),
		Runner:          ocrRunner,
		Publisher:       pub,
		Writeback:       writeback.NewWritebackPublisherAdapter(writebackOrchestrator),
		PreviewRunner:   ocrRunner,
		ProgressManager: progressManager,
	})

	// Review handler (review + platform)
	review_service.InitHandler(reviewService, gitProviderSvc)

	// Webhook handler
	webhookSvc := webhook.NewService(webhook.ServiceConfig{
		Normalizers: map[string]webhook.Normalizer{
			"github":       &webhook.GitHubNormalizer{},
			"gitlab":       &webhook.GitLabNormalizer{},
			"gitea":        &webhook.GiteaNormalizer{},
			"gitcode":      &webhook.GitCodeNormalizer{},
			"tencent_code": &webhook.TencentCodeNormalizer{},
		},
		DeliveryRepo:       repos.WebhookDelivery,
		ChangeRevisionRepo: repos.ChangeRevision,
		MaxPayloadBytes:    1 << 20,
	})
	review_service.InitWebhookHandler(webhookSvc, reviewService, repos.GitPlatform)

	// Settings handler (OCR, projects, platforms, rules, agents)
	settings_service.InitHandler(
		repos.OCRSettings,
		repos.ProjectPolicy,
		repos.GitPlatform,
		repos.GlobalRules,
		repos.ReviewerAgent,
		gitProviderSvc,
		ocrRunner,
	)

	// Stats handler
	statsService := statsSvc.NewService(repos.ContributorStats)
	stats_service.InitHandler(statsService)

	// Session handler
	sessionRepo := postgres.NewPostgresSessionRepository(db)
	sessionService := session.NewSessionService(sessionRepo)
	session_service.InitHandler(sessionService)

	// Agent manager
	agentManager := agent.NewAgentManager(agent.DefaultManagerConfig(), agent.NewDefaultAggregator(agent.DefaultAggregatorConfig()))
	runnerBaseConfig := runner.OpenCodeReviewConfig{
		Binary:          cfg.OCR.Binary,
		RulePath:        cfg.OCR.Rule,
		DefaultSeverity: "major",
		Concurrency:     2,
		MaxGitProcs:     4,
		TimeoutMinutes:  10,
	}
	runnerFactory := agent.NewOCRRunnerFactory(runnerBaseConfig)
	agentLoader := agent.NewAgentLoader(agentManager, runnerFactory, 10*time.Minute)

	enabledAgents, err := repos.ReviewerAgent.ListEnabled(ctx)
	if err != nil {
		hlog.Warnf("failed to load agents from database: %v", err)
	} else if len(enabledAgents) > 0 {
		if err := agentLoader.LoadFromDB(ctx, enabledAgents); err != nil {
			hlog.Warnf("failed to load agents: %v", err)
		}
	} else {
		ocrSettings, err := repos.OCRSettings.Get(ctx)
		if err == nil && ocrSettings != nil && ocrSettings.Background != "" {
			agentLoader.LoadDefaultAgent(ocrSettings.Background)
		} else {
			agentLoader.LoadDefaultAgent("")
		}
	}

	// Progress handler
	progress_service.InitHandler(progressManager)

	// Review map handler
	review_map_service.InitHandler(sessionService, repos.ReviewMap)

	// Publisher handler
	publisher_service.InitHandler(writebackOrchestrator)

	// Project agent handler
	project_agent_service.InitHandler(repos.ProjectAgent, repos.ReviewerAgent)

	h := createServer(cfg)
	registerGlobalMiddleware(h, cfg)

	// 注册路由（health + swagger + IDL 生成的路由）
	registerRoutes(h, db)

	app.GoBackground("review-workers", func(workerCtx context.Context) {
		reviewService.RunWorkers(
			workerCtx,
			cfg.Review.WorkerConcurrency,
			cfg.GetWorkerPollInterval(),
			cfg.GetWorkerStaleAfter(),
		)
	})
	hlog.Infof("background workers started with concurrency=%d", cfg.Review.WorkerConcurrency)

	app.AddResource("hertz", func(ctx context.Context) error {
		return h.Shutdown(ctx)
	})

	app.AddResource("progress-manager", func(ctx context.Context) error {
		progressManager.Close()
		return nil
	})

	hlog.Infof("review service starting on port %d", cfg.Port)
	return h.Run()
}

func initGitProviders(ctx context.Context, cfg *config.Config, repos *repository.RepoContainer) (*gitProvider.GitProviderService, error) {
	providerConfigs := make(map[string]gitplatform.Config)

	for name, gitCfg := range cfg.Git.Platforms {
		if gitCfg.Enabled && gitCfg.Token != "" {
			providerConfigs[name] = gitplatform.Config{
				Platform: gitplatform.Platform(name),
				BaseURL:  gitCfg.BaseURL,
				Token:    gitCfg.Token,
			}
		}
	}

	dbPlatforms, err := repos.GitPlatform.ListEnabled(ctx)
	if err != nil {
		hlog.Errorf("init: list enabled git platforms from DB: %v", err)
	}
	for _, p := range dbPlatforms {
		providerConfigs[p.Provider] = gitplatform.Config{
			Platform: gitplatform.Platform(p.Provider),
			BaseURL:  p.BaseURL,
			Token:    p.Token,
		}
	}

	for name, pCfg := range providerConfigs {
		existing, err := repos.GitPlatform.Get(ctx, name)
		if err != nil {
			hlog.Errorf("init: get git platform from DB %s: %v", name, err)
			continue
		}
		if existing == nil {
			if err := repos.GitPlatform.Save(ctx, &models.GitPlatform{
				Provider: name,
				BaseURL:  pCfg.BaseURL,
				Token:    pCfg.Token,
				Enabled:  true,
			}); err != nil {
				hlog.Errorf("init: save git platform to DB %s: %v", name, err)
			}
		}
	}

	return gitProvider.NewGitProviderService(providerConfigs)
}
