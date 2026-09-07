package modules

import (
	"context"

	"github.com/yi-nology/code-review-service/internal/repository"
	gitProvider "github.com/yi-nology/code-review-service/internal/service/provider"
	"github.com/yi-nology/code-review-service/internal/service/writeback"

	"github.com/oekit/logx"
	"go.uber.org/fx"
)

var WritebackModule = fx.Options(
	fx.Provide(NewWritebackOrchestrator),
	fx.Provide(NewWritebackPublisherAdapter),
)

func NewWritebackOrchestrator(
	gitProviderSvc *gitProvider.GitProviderService,
	repos *repository.RepoContainer,
	gerritContainer *GerritClientContainer,
) *writeback.Orchestrator {
	// 从 DB 加载策略配置，使用内置默认值兜底。
	maxComments := 20
	summaryOnly := false

	if repos != nil && repos.OCRSettings != nil {
		if s, err := repos.OCRSettings.Get(context.Background()); err == nil && s != nil {
			if s.PolicyMaxComments > 0 {
				maxComments = s.PolicyMaxComments
			}
			summaryOnly = s.PolicySummaryOnly
		}
	}

	// 平台差异收敛在 writeback.AdapterOptions（行号映射/降级策略）。
	adapters := map[string]writeback.Adapter{
		"github": writeback.NewPlatformAdapter(gitProviderSvc, writeback.AdapterOptions{
			Platform: "github",
		}),
		"gitlab": writeback.NewPlatformAdapter(gitProviderSvc, writeback.AdapterOptions{
			Platform:       "gitlab",
			InlineFallback: true,
		}),
		"gitea": writeback.NewPlatformAdapter(gitProviderSvc, writeback.AdapterOptions{
			Platform:     "gitea",
			NoteFallback: true,
		}),
		"forgejo": writeback.NewPlatformAdapter(gitProviderSvc, writeback.AdapterOptions{
			Platform:     "forgejo",
			NoteFallback: true,
		}),
		"gitcode": writeback.NewPlatformAdapter(gitProviderSvc, writeback.AdapterOptions{
			Platform:     "gitcode",
			NoteFallback: true,
		}),
		"tencent_code": writeback.NewPlatformAdapter(gitProviderSvc, writeback.AdapterOptions{
			Platform:       "tencent_code",
			InlineFallback: true,
		}),
	}

	// Gerrit 走独立的 go-gerrit 客户端，不经过 SDK。
	if gerritClient := gerritContainer.Get(); gerritClient != nil {
		adapters["gerrit"] = writeback.NewGerritAdapter(gerritClient)
		logx.Info("gerrit writeback adapter registered")
	}

	return writeback.NewOrchestrator(writeback.OrchestratorConfig{
		Adapters:    adapters,
		ItemRepo:    repos.WritebackItem,
		ChangeRepo:  repos.ChangeRevision,
		MaxComments: maxComments,
		SummaryOnly: summaryOnly,
	})
}

func NewWritebackPublisherAdapter(orch *writeback.Orchestrator) *writeback.WritebackPublisherAdapter {
	return writeback.NewWritebackPublisherAdapter(orch)
}
