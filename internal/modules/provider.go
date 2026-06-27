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
)

func NewGitProviderService(lc fx.Lifecycle, cfg *config.Config, repos *repository.RepoContainer) (*gitProvider.GitProviderService, error) {
	// 仅从 YAML/ENV 构造初始平台配置；DB 平台在 OnStart 阶段注册。
	// 构造保持纯净（无 DB 副作用），便于测试与失败定位。
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

	svc, err := gitProvider.NewGitProviderService(providerConfigs)
	if err != nil {
		return nil, fmt.Errorf("init git providers: %w", err)
	}

	// 数据库读写（持久化 YAML 平台、注册 DB 平台）放在 OnStart，
	// 使用 lifecycle ctx 而非 context.Background，构造期不产生副作用。
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			syncGitPlatforms(ctx, repos, svc, providerConfigs)
			logx.Infof("git providers initialized: %v", svc.GetSupportedPlatforms())
			return nil
		},
	})

	return svc, nil
}

// syncGitPlatforms 将 DB 中已启用的平台注册到服务，并把 YAML/ENV 配置的平台
// 持久化到 DB（缺失时写入），最后用 AddOrUpdateFromDB 确保 SkipTLS 通过 per-request 生效。
func syncGitPlatforms(ctx context.Context, repos *repository.RepoContainer, svc *gitProvider.GitProviderService, yamlConfigs map[string]gitplatform.Config) {
	dbPlatforms, err := repos.GitPlatform.ListEnabled(ctx)
	if err != nil {
		logx.Errorf("init: list enabled git platforms from DB: %v", err)
	}
	for _, p := range dbPlatforms {
		if p.SkipTLS {
			logx.Infof("init: platform %s configured with SkipTLS (per-request InsecureSkipTLS)", p.Provider)
		}
	}

	// 持久化 YAML/ENV 配置的平台到 DB（仅当不存在时）。
	for name, pCfg := range yamlConfigs {
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

	// 用 AddOrUpdateFromDB 重新注册所有 DB 平台（确保 SkipTLS 通过 InsecureSkipTLS 生效）。
	for _, p := range dbPlatforms {
		if err := svc.AddOrUpdateFromDB(&p); err != nil {
			logx.Errorf("init: re-register platform %s failed: %v", p.Provider, err)
		}
	}
}

func NewGitProviderProviders(svc *gitProvider.GitProviderService, cfg *config.Config) *gitProvider.GitProviderProviders {
	return gitProvider.NewGitProviderProviders(svc, int64(cfg.Review.MaxDiffBytes))
}
