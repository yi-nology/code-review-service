package modules

import (
	"context"
	"sync"

	"github.com/yi-nology/code-review-service/internal/repository"
	gerrit "github.com/yi-nology/code-review-service/internal/service/gerrit"

	"github.com/oekit/logx"
	"go.uber.org/fx"
)

// GerritModule 提供 Gerrit 客户端的 fx 模块。
// 客户端在构造期（Provide 阶段）从 DB 平台配置初始化，
// 确保在 WritebackOrchestrator 构造时已经可用。
var GerritModule = fx.Options(
	fx.Provide(NewGerritClientContainer),
)

// GerritClientContainer 持有 Gerrit 客户端实例。
// 支持热更新：当 DB 中的 Gerrit 平台配置变更时可调用 Reload。
type GerritClientContainer struct {
	mu     sync.RWMutex
	client *gerrit.Client
}

// Get 返回当前的 Gerrit 客户端，未配置时返回 nil。
func (c *GerritClientContainer) Get() *gerrit.Client {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.client
}

// Reload 用新的配置重建客户端。
func (c *GerritClientContainer) Reload(baseURL, token string, skipTLS bool) error {
	cl, err := gerrit.NewClient(baseURL, token, skipTLS)
	if err != nil {
		return err
	}
	c.mu.Lock()
	c.client = cl
	c.mu.Unlock()
	return nil
}

// NewGerritClientContainer 在构造期从 DB 读取 Gerrit 平台配置并初始化客户端。
// 由于 GerritModule 在 init.go 中排在 DatabaseModule/RepositoryModule 之后，
// DB 在构造期已可用。这保证了 Gerrit 客户端在 WritebackOrchestrator 构造时已经就绪。
func NewGerritClientContainer(repos *repository.RepoContainer) *GerritClientContainer {
	container := &GerritClientContainer{}

	if repos == nil || repos.GitPlatform == nil {
		logx.Info("gerrit: no repository container, skipping client init")
		return container
	}

	// 直接在构造期读取 DB，不依赖 fx OnStart hook。
	// 这避免了 fx 生命周期排序问题：WritebackOrchestrator 在 Provide 阶段
	// 需要 GerritClientContainer 已经有 client。
	platforms, err := repos.GitPlatform.ListEnabled(context.Background())
	if err != nil {
		logx.Errorf("gerrit: list enabled platforms from DB: %v", err)
		return container
	}

	for _, p := range platforms {
		if p.Provider != "gerrit" {
			continue
		}
		if p.BaseURL == "" || p.Token == "" {
			logx.Warn("gerrit: platform configured but missing base_url or token")
			continue
		}

		cl, err := gerrit.NewClient(p.BaseURL, p.Token, p.SkipTLS)
		if err != nil {
			logx.Errorf("gerrit: init client for %s: %v", p.BaseURL, err)
			continue
		}

		container.client = cl
		logx.Info("gerrit: client initialized",
			"base_url", p.BaseURL,
			"skip_tls", p.SkipTLS,
		)
		return container
	}

	logx.Info("gerrit: no platform configured in DB")
	return container
}
