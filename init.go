package main

import (
	"context"
	"fmt"

	"review-service/biz/handler/review_service"
	"review-service/internal/config"
	"review-service/internal/repository"
	"review-service/internal/repository/postgres"
	"review-service/internal/repository/postgres/models"
	"review-service/internal/service/policy"
	gitProvider "review-service/internal/service/provider"
	"review-service/internal/service/publisher"
	"review-service/internal/service/runner"
	svcReview "review-service/internal/service/review"
	statsSvc "review-service/internal/service/stats"
	"review-service/internal/service/webhook"
	"review-service/internal/service/writeback"

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

	h := createServer(cfg)
	registerGlobalMiddleware(h, cfg)

	registerCustomRoutes(h, cfg, db, handler, webhookHandler, statsHandler)

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
