package modules

import (
	"context"
	"time"

	"github.com/yi-nology/code-review-service/internal/ocr"
	"github.com/yi-nology/code-review-service/internal/repository"
	"github.com/yi-nology/code-review-service/internal/service/agent"

	"github.com/oekit/concurrency/async"
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

func InitAgentManager(lc fx.Lifecycle, repos *repository.RepoContainer, agentMgr *agent.AgentManager) {
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			// PR-21 修复：原 OnStart 同步执行 OCRSettings.Get + ReviewerAgent.ListEnabled，
			// 慢 DB 场景下阻塞 fx 启动（5s+）。
			// 改为 async.GoSafe：OnStart 立即返回，DB 加载在后台协程完成。
			// 评审请求到达时若 loader 还没就绪，会因「无 agent」走 no-agent 路径（见 executor），
			// 不会 panic。
			async.GoSafe(func() {
				// 拷贝 ctx 防止外层取消影响加载
				loadCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()
				syncAgentManager(loadCtx, repos, agentMgr)
			})
			return nil
		},
	})
}

// syncAgentManager 是从原 OnStart 拆出的同步加载逻辑，被 async.GoSafe 调用。
// 失败只 logx.Warn，不影响启动。
func syncAgentManager(ctx context.Context, repos *repository.RepoContainer, agentMgr *agent.AgentManager) {
	var reasoningExtraBody string
	ocrTimeoutMinutes := 30
	ocrConcurrency := 2
	ocrMaxGitProcs := 4
	ocrDefaultSeverity := "major"
	ocrBinary := "ocr"
	llmURL, llmModel, llmAuthToken, llmAuthHeader, llmLanguage := "", "", "", "", ""
	if ocrSettings, err := repos.OCRSettings.Get(ctx); err == nil && ocrSettings != nil {
		mode := ocrSettings.ReasoningMode
		if mode == "" {
			mode = "light"
		}
		reasoningExtraBody = ocr.ResolveExtraBody(string(ocrSettings.ExtraBody), mode, ocrSettings.ReasoningBudget, ocrSettings.Provider)
		if ocrSettings.TimeoutMinutes > 0 {
			ocrTimeoutMinutes = ocrSettings.TimeoutMinutes
		}
		if ocrSettings.Concurrency > 0 {
			ocrConcurrency = ocrSettings.Concurrency
		}
		if ocrSettings.MaxGitProcs > 0 {
			ocrMaxGitProcs = ocrSettings.MaxGitProcs
		}
		if ocrSettings.DefaultSeverity != "" {
			ocrDefaultSeverity = ocrSettings.DefaultSeverity
		}
		if ocrSettings.Binary != "" {
			ocrBinary = ocrSettings.Binary
		}
		llmURL = ocrSettings.URL
		llmModel = ocrSettings.ModelName
		llmAuthToken = ocrSettings.AuthToken
		llmAuthHeader = ocrSettings.AuthHeader
		llmLanguage = ocrSettings.Language
	}

	runnerFactory := agent.NewOCRRunnerFactory(ocr.Config{
		Binary:          ocrBinary,
		DefaultSeverity: ocrDefaultSeverity,
		Concurrency:     ocrConcurrency,
		MaxGitProcs:     ocrMaxGitProcs,
		TimeoutMinutes:  ocrTimeoutMinutes,
		ExtraBody:       reasoningExtraBody,
		URL:             llmURL,
		Model:           llmModel,
		AuthToken:       llmAuthToken,
		AuthHeader:      llmAuthHeader,
		Language:        llmLanguage,
	})
	agentMgr.SetFactory(runnerFactory)
	agentMgr.SetProjectResolver(&projectAgentResolverAdapter{repo: repos.ProjectAgent})
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
}
