package modules

import (
	"context"
	"fmt"

	"github.com/yi-nology/code-review-service/internal/config"
	"github.com/yi-nology/code-review-service/internal/repository"
	gitProvider "github.com/yi-nology/code-review-service/internal/service/provider"

	"github.com/oekit/logx"
	"go.uber.org/fx"
)

var ProviderModule = fx.Options(
	fx.Provide(NewGitProviderService),
	fx.Provide(NewGitProviderProviders),
)

func NewGitProviderService(lc fx.Lifecycle, repos *repository.RepoContainer) (*gitProvider.GitProviderService, error) {
	// Git 平台配置全部从 DB 加载，不再从 YAML 读取。
	svc, err := gitProvider.NewGitProviderService(nil)
	if err != nil {
		return nil, fmt.Errorf("init git providers: %w", err)
	}

	// 数据库读写（注册 DB 平台）放在 OnStart，
	// 使用 lifecycle ctx 而非 context.Background，构造期不产生副作用。
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			syncGitPlatforms(ctx, repos, svc)
			logx.Infof("git providers initialized: %v", svc.GetSupportedPlatforms())
			return nil
		},
	})

	return svc, nil
}

// syncGitPlatforms 将 DB 中已启用的平台注册到服务，用 AddOrUpdateFromDB 确保 SkipTLS 通过 per-request 生效。
func syncGitPlatforms(ctx context.Context, repos *repository.RepoContainer, svc *gitProvider.GitProviderService) {
	dbPlatforms, err := repos.GitPlatform.ListEnabled(ctx)
	if err != nil {
		logx.Errorf("init: list enabled git platforms from DB: %v", err)
	}
	for _, p := range dbPlatforms {
		if p.SkipTLS {
			logx.Infof("init: platform %s configured with SkipTLS (per-request InsecureSkipTLS)", p.Provider)
		}
	}

	// 用 AddOrUpdateFromDB 注册所有 DB 平台（确保 SkipTLS 通过 InsecureSkipTLS 生效）。
	for _, p := range dbPlatforms {
		if err := svc.AddOrUpdateFromDB(&p); err != nil {
			logx.Errorf("init: re-register platform %s failed: %v", p.Provider, err)
		}
	}
}

func NewGitProviderProviders(cfg *config.Config, svc *gitProvider.GitProviderService, repos *repository.RepoContainer) *gitProvider.GitProviderProviders {
	// 优先级：DB OCR settings > yaml review.max_diff_bytes > 代码默认值。
	// 此前 yaml 键被解析但从未接线，prod 配 10MiB 实际跑 5MiB 默认值。
	maxDiffBytes := gitProvider.DefaultMaxDiffBytes
	if cfg != nil && cfg.Review.MaxDiffBytes > 0 {
		maxDiffBytes = int64(cfg.Review.MaxDiffBytes)
	}
	if repos != nil && repos.OCRSettings != nil {
		if s, err := repos.OCRSettings.Get(context.Background()); err == nil && s != nil && s.MaxDiffBytes > 0 {
			maxDiffBytes = s.MaxDiffBytes
		}
	}

	return gitProvider.NewGitProviderProviders(svc, maxDiffBytes)
}
