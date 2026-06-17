package modules

import (
	"context"
	"time"

	"github.com/yi-nology/code-review-service/internal/config"
	"github.com/yi-nology/code-review-service/internal/repository"
	"github.com/yi-nology/code-review-service/internal/service/agent"
	"github.com/yi-nology/code-review-service/internal/service/runner"

	"github.com/cloudwego/hertz/pkg/common/hlog"
	"go.uber.org/fx"
)

var AgentModule = fx.Options(
	fx.Provide(NewAgentManager),
	fx.Invoke(InitAgentManager),
)

func NewAgentManager() *agent.AgentManager {
	return agent.NewAgentManager(agent.DefaultManagerConfig(), agent.NewDefaultAggregator(agent.DefaultAggregatorConfig()))
}

func InitAgentManager(lc fx.Lifecycle, cfg *config.Config, repos *repository.RepoContainer, agentMgr *agent.AgentManager) {
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			runnerFactory := agent.NewOCRRunnerFactory(runner.OpenCodeReviewConfig{
				Binary:          cfg.OCR.Binary,
				RulePath:        cfg.OCR.Rule,
				DefaultSeverity: "major",
				Concurrency:     2,
				MaxGitProcs:     4,
				TimeoutMinutes:  10,
			})
			agentLoader := agent.NewAgentLoader(agentMgr, runnerFactory, 10*time.Minute)

			enabledAgents, err := repos.ReviewerAgent.ListEnabled(ctx)
			if err != nil {
				hlog.Warnf("failed to load agents from database: %v", err)
			} else if len(enabledAgents) > 0 {
				if err := agentLoader.LoadFromDB(ctx, enabledAgents); err != nil {
					hlog.Warnf("failed to load agents: %v", err)
				}
			} else {
				ocrSettings, err := repos.OCRSettings.Get(ctx)
				if err == nil && ocrSettings != nil && ocrSettings.Background != "" {
					agentLoader.LoadDefaultAgent(ocrSettings.Background)
				} else {
					agentLoader.LoadDefaultAgent("")
				}
			}
			return nil
		},
	})
}
