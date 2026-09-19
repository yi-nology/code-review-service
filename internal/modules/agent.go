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

// NewAgentManager 构造装配完整的 manager：runner 工厂（ConfigResolver 热更新）与
// 项目解析器在 DI 期注入；仅「agent 名册加载 + 超时同步」仍由 InitAgentManager
// 在后台完成（PR-21：不能阻塞 fx 启动）。
func NewAgentManager(repos *repository.RepoContainer, resolver *ocr.ConfigResolver) *agent.AgentManager {
	return agent.NewWiredAgentManager(
		agent.DefaultManagerConfig(),
		agent.NewDefaultAggregator(agent.DefaultAggregatorConfig()),
		agent.ManagerDeps{
			Factory:         agent.NewResolvingOCRRunnerFactory(resolver),
			ProjectResolver: &projectAgentResolverAdapter{repo: repos.ProjectAgent},
		},
	)
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

func InitAgentManager(lc fx.Lifecycle, repos *repository.RepoContainer, resolver *ocr.ConfigResolver, agentMgr *agent.AgentManager) {
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

// syncAgentManager 后台完成启动期才需要的收尾：超时同步与 agent 名册加载。
// 失败只 logx.Warn，不影响启动。
func syncAgentManager(ctx context.Context, repos *repository.RepoContainer, agentMgr *agent.AgentManager) {
	// 超时推导需要当前的 TimeoutMinutes；agent/总超时是 manager 状态，
	// 无需跟 runner 配置一起热更新（评审执行中不变），启动读一次即可。
	if ocrSettings, err := repos.OCRSettings.Get(ctx); err == nil && ocrSettings != nil && ocrSettings.TimeoutMinutes > 0 {
		agentMgr.UpdateTimeouts(time.Duration(ocrSettings.TimeoutMinutes) * time.Minute)
		logx.Infof("agent manager 超时已同步: OCR=%dm, agent=%dm, total=%dm",
			ocrSettings.TimeoutMinutes, ocrSettings.TimeoutMinutes+5, ocrSettings.TimeoutMinutes+10)
	}

	enabledAgents, err := repos.ReviewerAgent.ListEnabled(ctx)
	switch {
	case err != nil:
		logx.Warnf("failed to load agents from database: %v", err)
	case len(enabledAgents) > 0:
		if err := agentMgr.LoadAgents(ctx, enabledAgents); err != nil {
			logx.Warnf("failed to load agents: %v", err)
		}
	default:
		ocrSettings, err := repos.OCRSettings.Get(ctx)
		if err == nil && ocrSettings != nil && ocrSettings.Background != "" {
			agentMgr.LoadDefaultAgent(ocrSettings.Background)
		} else {
			agentMgr.LoadDefaultAgent("")
		}
	}
}
