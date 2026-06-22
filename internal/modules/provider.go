package modules

import (
	"context"
	"fmt"
	"os"

	"github.com/yi-nology/code-review-service/internal/config"
	"github.com/yi-nology/code-review-service/internal/repository"
	"github.com/yi-nology/code-review-service/internal/repository/postgres/models"
	gitProvider "github.com/yi-nology/code-review-service/internal/service/provider"

	"github.com/cloudwego/hertz/pkg/common/hlog"
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
		hlog.Errorf("init: list enabled git platforms from DB: %v", err)
	}
	for _, p := range dbPlatforms {
		providerConfigs[p.Provider] = gitplatform.Config{
			Platform: gitplatform.Platform(p.Provider),
			BaseURL:  p.BaseURL,
			Token:    p.Token,
			SkipTLS:  p.SkipTLS,
		}
		// 自签名证书的平台：git clone 子进程也需要跳过 SSL 验证
		if p.SkipTLS {
			os.Setenv("GIT_SSL_NO_VERIFY", "true")
			hlog.Infof("init: GIT_SSL_NO_VERIFY enabled for platform %s (self-signed cert)", p.Provider)
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

	svc, err := gitProvider.NewGitProviderService(providerConfigs)
	if err != nil {
		return nil, fmt.Errorf("init git providers: %w", err)
	}

	// 用 AddOrUpdateFromDB 重新注册所有 DB 平台（确保 SkipTLS + GIT_SSL_NO_VERIFY 生效）
	for _, p := range dbPlatforms {
		if p.SkipTLS {
			os.Setenv("GIT_SSL_NO_VERIFY", "true")
		}
		if err := svc.AddOrUpdateFromDB(&p); err != nil {
			hlog.Errorf("init: re-register platform %s failed: %v", p.Provider, err)
		}
	}

	hlog.Infof("git providers initialized: %v", svc.GetSupportedPlatforms())
	return svc, nil
}

func NewGitProviderProviders(svc *gitProvider.GitProviderService, cfg *config.Config) *gitProvider.GitProviderProviders {
	return gitProvider.NewGitProviderProviders(svc, int64(cfg.Review.MaxDiffBytes))
}

func NewGitProviderAdapter(svc *gitProvider.GitProviderService) *gitProvider.GitProviderAdapter {
	return gitProvider.NewGitProviderAdapter(svc)
}
