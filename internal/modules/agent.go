package modules

import (
	"context"
	"time"

	"github.com/yi-nology/code-review-service/internal/config"
	"github.com/yi-nology/code-review-service/internal/repository"
	"github.com/yi-nology/code-review-service/internal/service/agent"
	"github.com/yi-nology/code-review-service/internal/service/runner"

	"github.com/oekit/logx"
	"go.uber.org/fx"
)

var AgentModule = fx.Options(
	fx.Provide(NewAgentManager),
	fx.Invoke(InitAgentManager),
)

func NewAgentManager() *agent.AgentManager {
	return agent.NewAgentManager(agent.DefaultManagerConfig(), agent.NewDefaultAggregator(agent.DefaultAggregatorConfig()))
}

// projectAgentResolverAdapter 把 ProjectAgentRepository 适配成 agent.ProjectAgentResolver。
// 返回 persona(name) → 项目优先级映射；空 map 表示该项目未配置任何 project_agents。
type projectAgentResolverAdapter struct {
	repo repository.ProjectAgentRepository
}

func (a *projectAgentResolverAdapter) ResolveProjectAgents(ctx context.Context, provider, repo string) (map[string]int, error) {
	projectAgents, err := a.repo.GetEnabledByProject(ctx, provider, repo)
	if err != nil {
		return nil, err
	}
	if len(projectAgents) == 0 {
		return nil, nil
	}
	m := make(map[string]int, len(projectAgents))
	for _, pa := range projectAgents {
		// AgentName 来自 GetEnabledByProject 的 join；缺失则跳过
		if pa.AgentName == "" {
			continue
		}
		m[pa.AgentName] = pa.Priority
	}
	return m, nil
}

func InitAgentManager(lc fx.Lifecycle, cfg *config.Config, repos *repository.RepoContainer, agentMgr *agent.AgentManager) {
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			// 读取 OCRSettings 获取推理模式 + 超时配置。
			// 超时必须同步到 runner 工厂与 agent 框架，否则用户改 OCRSettings.TimeoutMinutes
			// 后 agent 仍按旧超时杀掉 OCR（#25）。
			var reasoningExtraBody string
			ocrTimeoutMinutes := 30
			if ocrSettings, err := repos.OCRSettings.Get(ctx); err == nil && ocrSettings != nil {
				mode := ocrSettings.ReasoningMode
				if mode == "" {
					mode = "light"
				}
				reasoningExtraBody = runner.ResolveExtraBody(string(ocrSettings.ExtraBody), mode, ocrSettings.ReasoningBudget, ocrSettings.Provider)
				if ocrSettings.TimeoutMinutes > 0 {
					ocrTimeoutMinutes = ocrSettings.TimeoutMinutes
				}
			}

			runnerFactory := agent.NewOCRRunnerFactory(runner.OpenCodeReviewConfig{
				Binary:          cfg.OCR.Binary,
				RulePath:        cfg.OCR.Rule,
				DefaultSeverity: "major",
				Concurrency:     2,
				MaxGitProcs:     4,
				TimeoutMinutes:  ocrTimeoutMinutes,
				ExtraBody:       reasoningExtraBody,
			})
			agentMgr.SetFactory(runnerFactory) // 阶段 5：使 manager 具备构造 ephemeral agent 的能力
			// 注入项目级 agent 解析器：使审查按项目裁剪（per-project 白名单覆盖 / 缺省自动推断）
			agentMgr.SetProjectResolver(&projectAgentResolverAdapter{repo: repos.ProjectAgent})
			// 同步 agent 框架超时 = OCR 超时 + buffer（agent 超时必须大于 OCR 单次超时）
			agentMgr.UpdateTimeouts(time.Duration(ocrTimeoutMinutes) * time.Minute)
			logx.Infof("agent manager 超时已同步: OCR=%dm, agent=%dm, total=%dm", ocrTimeoutMinutes, ocrTimeoutMinutes+5, ocrTimeoutMinutes+10)
			agentLoader := agent.NewAgentLoader(agentMgr, runnerFactory, 10*time.Minute)

			enabledAgents, err := repos.ReviewerAgent.ListEnabled(ctx)
			switch {
			case err != nil:
				logx.Warnf("failed to load agents from database: %v", err)
			case len(enabledAgents) > 0:
				if err := agentLoader.LoadFromDB(ctx, enabledAgents); err != nil {
					logx.Warnf("failed to load agents: %v", err)
				}
			default:
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
