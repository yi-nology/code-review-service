package modules

import (
	"context"

	domainplatform "github.com/yi-nology/code-review-service/internal/domain/platform"
	"github.com/yi-nology/code-review-service/internal/repository"
	"github.com/yi-nology/code-review-service/internal/service/gerrit"

	"github.com/oekit/logx"
	"go.uber.org/fx"
)

// GerritModule 提供 Gerrit 客户端持有者的 fx 模块。
// 客户端在构造期（Provide 阶段）从 DB 平台配置初始化，
// 确保在 WritebackOrchestrator 构造时已经可用。
var GerritModule = fx.Options(
	fx.Provide(NewGerritClientHolder),
)

// NewGerritClientHolder 在构造期从 DB 读取 Gerrit 平台配置并初始化客户端。
//
// 由于 GerritModule 在 init.go 中排在 DatabaseModule/RepositoryModule 之后，
// DB 在构造期已可用。这保证了 Gerrit 客户端在 WritebackOrchestrator 构造时已经就绪。
//
// holder 本身放在 service 层（internal/service/gerrit）：web console 改平台配置后
// 要能热更新客户端，而 biz/handler 不能 import internal/modules（会与装配层成环），
// 路径必须走得到 service 层。
func NewGerritClientHolder(repos *repository.RepoContainer) *gerrit.ClientHolder {
	holder := &gerrit.ClientHolder{}

	if repos == nil || repos.GitPlatform == nil {
		logx.Info("gerrit: no repository container, skipping client init")
		return holder
	}

	// 直接在构造期读取 DB，不依赖 fx OnStart hook。
	// 这避免了 fx 生命周期排序问题：WritebackOrchestrator 在 Provide 阶段
	// 需要 Gerrit 客户端持有者已经就绪。
	platforms, err := repos.GitPlatform.ListEnabled(context.Background())
	if err != nil {
		logx.Errorf("gerrit: list enabled platforms from DB: %v", err)
		return holder
	}

	for _, p := range platforms {
		if p.Provider != domainplatform.ProviderGerrit {
			continue
		}
		if p.BaseURL == "" || p.Token == "" {
			logx.Warn("gerrit: platform configured but missing base_url or token")
			continue
		}

		if err := holder.Reload(p.BaseURL, p.Token, p.SkipTLS); err != nil {
			logx.Errorf("gerrit: init client for %s: %v", p.BaseURL, err)
			continue
		}

		logx.Info("gerrit: client initialized",
			"base_url", p.BaseURL,
			"skip_tls", p.SkipTLS,
		)
		return holder
	}

	logx.Info("gerrit: no platform configured in DB")
	return holder
}
