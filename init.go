package main

import (
	"context"
	"fmt"
	"time"

	"github.com/yi-nology/code-review-service/biz/handler/review_service"
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
	})

	handler := review_service.NewReviewHandler(
		reviewService,
		repos.OCRSettings,
		repos.ProjectPolicy,
		repos.GitPlatform,
		repos.GlobalRules,
		repos.ReviewerAgent,
		gitProviderSvc,
		ocrRunner,
	)

	webhookSvc := webhook.NewService(webhook.ServiceConfig{
		Normalizers: map[string]webhook.Normalizer{
			"github": &webhook.GitHubNormalizer{},
			"gitlab": &webhook.GitLabNormalizer{},
			"gitea":  &webhook.GiteaNormalizer{},
		},
		DeliveryRepo:       repos.WebhookDelivery,
		ChangeRevisionRepo: repos.ChangeRevision,
		MaxPayloadBytes:    1 << 20,
	})

	webhookHandler := review_service.NewWebhookHandler(
		webhookSvc,
		reviewService,
		repos.GitPlatform,
	)

	statsService := statsSvc.NewService(repos.ContributorStats)
	statsHandler := review_service.NewStatsHandler(statsService)

	// Initialize session service
	sessionRepo := postgres.NewPostgresSessionRepository(db)
	sessionService := session.NewSessionService(sessionRepo)
	sessionHandler := review_service.NewSessionHandler(sessionService)

	// Initialize agent manager and load agents from database
	agentManager := agent.NewAgentManager(agent.DefaultManagerConfig(), agent.NewDefaultAggregator(agent.DefaultAggregatorConfig()))

	// Create runner factory
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

	// Load agents from database
	enabledAgents, err := repos.ReviewerAgent.ListEnabled(ctx)
	if err != nil {
		hlog.Warnf("failed to load agents from database: %v", err)
	} else if len(enabledAgents) > 0 {
		if err := agentLoader.LoadFromDB(ctx, enabledAgents); err != nil {
			hlog.Warnf("failed to load agents: %v", err)
		}
	} else {
		// Load default agent with OCRSettings background
		ocrSettings, err := repos.OCRSettings.Get(ctx)
		if err == nil && ocrSettings != nil && ocrSettings.Background != "" {
			agentLoader.LoadDefaultAgent(ocrSettings.Background)
		} else {
			agentLoader.LoadDefaultAgent("")
		}
	}

	// Initialize progress manager
	progressManager := progress.NewProgressManager()
	progressWSHandler := review_service.NewProgressWSHandler(progressManager)

	// Initialize review map handler
	reviewMapHandler := review_service.NewReviewMapHandler(sessionService, repos.ReviewMap)

	// Initialize publisher handler
	publisherHandler := review_service.NewPublisherHandler(writebackOrchestrator)

	// Initialize project agent handler
	projectAgentHandler := review_service.NewProjectAgentHandler(repos.ProjectAgent, repos.ReviewerAgent)

	h := createServer(cfg)
	registerGlobalMiddleware(h, cfg)

	registerCustomRoutes(h, cfg, db, handler, webhookHandler, statsHandler, sessionHandler, progressWSHandler, reviewMapHandler, publisherHandler, projectAgentHandler)

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
