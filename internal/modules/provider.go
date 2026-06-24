package modules

import (
	"context"
	"fmt"

	"github.com/yi-nology/code-review-service/internal/config"
	"github.com/yi-nology/code-review-service/internal/repository"
	"github.com/yi-nology/code-review-service/internal/repository/postgres/models"
	gitProvider "github.com/yi-nology/code-review-service/internal/service/provider"

	"github.com/oekit/logx"
	gitplatform "github.com/yi-nology/git-platform-sdk/provider"
	"go.uber.org/fx"
)

var ProviderModule = fx.Options(
	fx.Provide(NewGitProviderService),
	fx.Provide(NewGitProviderProviders),
	fx.Provide(NewGitProviderAdapter),
)

func NewGitProviderService(lc fx.Lifecycle, cfg *config.Config, repos *repository.RepoContainer) (*gitProvider.GitProviderService, error) {
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

	ctx := context.Background()
	dbPlatforms, err := repos.GitPlatform.ListEnabled(ctx)
	if err != nil {
		logx.Errorf("init: list enabled git platforms from DB: %v", err)
	}
	for _, p := range dbPlatforms {
		providerConfigs[p.Provider] = gitplatform.Config{
			Platform: gitplatform.Platform(p.Provider),
			BaseURL:  p.BaseURL,
			Token:    p.Token,
			SkipTLS:  p.SkipTLS,
		}
		// SkipTLS 通过 gitbackend InsecureSkipTLS per-request 传递，不再全局 os.Setenv。
		if p.SkipTLS {
			logx.Infof("init: platform %s configured with SkipTLS (per-request InsecureSkipTLS)", p.Provider)
		}
	}

	for name, pCfg := range providerConfigs {
		existing, err := repos.GitPlatform.Get(ctx, name, "default")
		if err != nil {
			logx.Errorf("init: get git platform from DB %s: %v", name, err)
			continue
		}
		if existing == nil {
			if err := repos.GitPlatform.Save(ctx, &models.GitPlatform{
				Provider: name,
				BaseURL:  pCfg.BaseURL,
				Token:    pCfg.Token,
				Enabled:  true,
			}); err != nil {
				logx.Errorf("init: save git platform to DB %s: %v", name, err)
			}
		}
	}

	svc, err := gitProvider.NewGitProviderService(providerConfigs)
	if err != nil {
		return nil, fmt.Errorf("init git providers: %w", err)
	}

	// 用 AddOrUpdateFromDB 重新注册所有 DB 平台（确保 SkipTLS 通过 InsecureSkipTLS 生效）
	for _, p := range dbPlatforms {
		if err := svc.AddOrUpdateFromDB(&p); err != nil {
			logx.Errorf("init: re-register platform %s failed: %v", p.Provider, err)
		}
	}

	logx.Infof("git providers initialized: %v", svc.GetSupportedPlatforms())
	return svc, nil
}

func NewGitProviderProviders(svc *gitProvider.GitProviderService, cfg *config.Config) *gitProvider.GitProviderProviders {
	return gitProvider.NewGitProviderProviders(svc, int64(cfg.Review.MaxDiffBytes))
}

func NewGitProviderAdapter(svc *gitProvider.GitProviderService) *gitProvider.GitProviderAdapter {
	return gitProvider.NewGitProviderAdapter(svc)
}
