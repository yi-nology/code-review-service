package modules

import (
	"github.com/yi-nology/code-review-service/internal/config"
	"github.com/yi-nology/code-review-service/internal/repository"
	"github.com/yi-nology/code-review-service/internal/service/llm"
	gitProvider "github.com/yi-nology/code-review-service/internal/service/provider"
	"github.com/yi-nology/code-review-service/internal/service/writeback"

	"go.uber.org/fx"
)

var WritebackModule = fx.Options(
	fx.Provide(NewWritebackOrchestrator),
	fx.Provide(NewWritebackPublisherAdapter),
	fx.Provide(NewWritebackTranslator),
)

func NewWritebackOrchestrator(
	cfg *config.Config,
	gitProviderSvc *gitProvider.GitProviderService,
	repos *repository.RepoContainer,
) *writeback.Orchestrator {
	return writeback.NewOrchestrator(writeback.OrchestratorConfig{
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
}

func NewWritebackPublisherAdapter(orch *writeback.Orchestrator) *writeback.WritebackPublisherAdapter {
	return writeback.NewWritebackPublisherAdapter(orch)
}

// NewWritebackTranslator 创建拟人化翻译器（阶段 4），复用 LLM Provider。
func NewWritebackTranslator(provider llm.Provider) *writeback.Translator {
	return writeback.NewTranslator(provider)
}
