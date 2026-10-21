package agent

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/oekit/concurrency/async"
	"github.com/oekit/logx"
	"github.com/oekit/pkg/slicex"
)

// AgentManager manages multiple review agents
type AgentManager struct {
	agents          []reviewAgent
	aggregator      ResultAggregator
	config          ManagerConfig
	factory         reviewRunnerFactory  // 用于构造 ephemeral agent 的 runner（阶段 5）
	projectResolver ProjectAgentResolver // 解析项目级 agent 覆盖（per-project 白名单）
	mu              sync.RWMutex
}

// ProjectAgentResolver 解析某项目配置的有效审查员集（persona → 项目优先级）。
// 返回 nil/空 map 表示该项目未配置任何 project_agents，调用方应走全局缺省逻辑
// （技术栈自动推断 → is_default 兜底 → 全部 enabled 兜底）。
type ProjectAgentResolver interface {
	ResolveProjectAgents(ctx context.Context, provider, repo string) (map[string]int, error)
}

// SetFactory 注入 runner 工厂，使 AgentManager 具备构造临时（ephemeral）agent 的能力。
func (m *AgentManager) SetFactory(factory reviewRunnerFactory) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.factory = factory
}

// SetProjectResolver 注入项目级 agent 解析器，使 AgentManager 能按项目裁剪审查员集。
func (m *AgentManager) SetProjectResolver(r ProjectAgentResolver) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.projectResolver = r
}

// UpdateTimeouts 根据用户配置的 OCR 超时同步 agent 框架超时。
// AgentTimeout 必须大于单次 OCR 超时，否则 OCR 还没跑完就被 agent 框架杀掉（#25：用户改
// OCRSettings.TimeoutMinutes 后，硬编码的 35min 会让大超时配置的评审被误杀）。
func (m *AgentManager) UpdateTimeouts(ocrTimeout time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.config.AgentTimeout = ocrTimeout + 5*time.Minute
	m.config.TotalTimeout = ocrTimeout + 10*time.Minute
}

// EphemeralSpec 描述一个临时的、不入库的 reviewer（阶段 5）。
type EphemeralSpec struct {
	Name       string // 实例名，需在本次评审内唯一
	Background string // 评审侧重点，透传 OCR --background
}

// ManagerConfig contains manager configuration
type ManagerConfig struct {
	MaxConcurrency int
	AgentTimeout   time.Duration
	TotalTimeout   time.Duration
}

// DefaultManagerConfig returns default manager configuration
func DefaultManagerConfig() ManagerConfig {
	return ManagerConfig{
		MaxConcurrency: 4,
		AgentTimeout:   35 * time.Minute,
		TotalTimeout:   40 * time.Minute,
	}
}

// NewAgentManager creates a new agent manager
func NewAgentManager(config ManagerConfig, aggregator ResultAggregator) *AgentManager {
	return &AgentManager{
		config:     config,
		aggregator: aggregator,
	}
}

// ManagerDeps 是构造即完整的装配参数：工厂/项目解析器/超时在构造期一次性注入，
// manager 从构造返回即可用（此前依赖 SetFactory/SetProjectResolver/UpdateTimeouts
// 三个 setter 在后台 goroutine 里补注，早到的评审会静默走 no-agent 路径）。
type ManagerDeps struct {
	Factory         reviewRunnerFactory
	ProjectResolver ProjectAgentResolver
	// OCRTimeout 用于推导 AgentTimeout/TotalTimeout；<=0 时保留 config 里的默认值。
	OCRTimeout time.Duration
}

// NewWiredAgentManager 构造装配完整的 AgentManager。
func NewWiredAgentManager(config ManagerConfig, aggregator ResultAggregator, deps ManagerDeps) *AgentManager {
	m := NewAgentManager(config, aggregator)
	if deps.OCRTimeout > 0 {
		m.config.AgentTimeout = deps.OCRTimeout + 5*time.Minute
		m.config.TotalTimeout = deps.OCRTimeout + 10*time.Minute
	}
	m.factory = deps.Factory
	m.projectResolver = deps.ProjectResolver
	return m
}

// RegisterAgent registers a review agent
func (m *AgentManager) RegisterAgent(agent reviewAgent) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.agents = append(m.agents, agent)
	logx.Infof("registered agent: %s (priority: %d)", agent.Name(), agent.Priority())
}

// GetAgents returns all registered agents
func (m *AgentManager) GetAgents() []reviewAgent {
	m.mu.RLock()
	defer m.mu.RUnlock()
	agents := make([]reviewAgent, len(m.agents))
	copy(agents, m.agents)
	return agents
}

// AgentNames 返回已注册 Agent 的名字列表（按注册顺序）。
//
// 它让消费方能只依赖「名册」而不依赖 Agent 的执行能力：评审前置编排只需要
// 把名册写进审计步骤，并不需要跑 Agent——因此 review 侧声明窄接口
// （review.AgentRegistry），实现方无需 import 声明方（Go 的接口是结构化的）。
func (m *AgentManager) AgentNames() []string {
	agents := m.GetAgents()
	names := make([]string, 0, len(agents))
	for _, a := range agents {
		names = append(names, a.Name())
	}
	return names
}

// getFilteredAgents returns agents matching the filter
func (m *AgentManager) getFilteredAgents(filter func(reviewAgent) bool) []reviewAgent {
	m.mu.RLock()
	agents := make([]reviewAgent, len(m.agents))
	copy(agents, m.agents)
	m.mu.RUnlock()

	if filter != nil {
		agents = slicex.Filter(agents, filter)
	}
	return agents
}

// RunAgentsForProject 按项目裁剪后运行审查员，实现三层优先级：

// AgentStatus represents the status of an agent
type AgentStatus struct {
	Name     string        `json:"name"`
	Status   string        `json:"status"`
	Progress float64       `json:"progress"`
	Message  string        `json:"message"`
	Duration time.Duration `json:"duration"`
	Error    string        `json:"error,omitempty"`
}

// RunAgentsForProject 按项目裁剪后运行审查员，实现三层优先级：
//  1. 项目配了 project_agents → 完全白名单覆盖（应用 project_priority）
//  2. 未配置 → 按工作区技术栈自动推断（always + stack:* 匹配）
//  3. 推断无匹配 → is_default=true 兜底；仍无 → 全部 enabled 兜底（防空跑）
//
// ephemeral 审查员始终追加。provider/repo 为空时等同于「未配置」，直接走层 2-4。
func (m *AgentManager) RunAgentsForProject(ctx context.Context, workspace Workspace, provider, repo string, ephemeral []EphemeralSpec, callback func(status AgentStatus)) (AggregatedResult, error) {
	agents := m.resolveAgentsForProject(ctx, workspace, provider, repo)
	return m.runAgents(ctx, agents, ephemeral, workspace, callback)
}

// resolveAgentsForProject 实现三层优先级解析，返回本次应运行的 agent 实例集。
func (m *AgentManager) resolveAgentsForProject(ctx context.Context, workspace Workspace, provider, repo string) []reviewAgent {
	// 层 1：项目级白名单（完全覆盖）
	if provider != "" && repo != "" {
		m.mu.RLock()
		resolver := m.projectResolver
		m.mu.RUnlock()
		if resolver != nil {
			projMap, err := resolver.ResolveProjectAgents(ctx, provider, repo)
			if err != nil {
				logx.Warnf("resolve project agents for %s/%s failed, fallback to auto-select: %v", provider, repo, err)
			} else if len(projMap) > 0 {
				selected := m.selectForProject(projMap)
				logx.Infof("project %s/%s override: running %d agents", provider, repo, len(selected))
				return selected
			}
		}
	}

	// 层 2：技术栈自动推断
	stacks := ProbeStacks(workspace.Dir)
	if selected := m.selectByStack(stacks); len(selected) > 0 {
		logx.Infof("auto-selected %d agents by stacks %v (no project config)", len(selected), stacks)
		return selected
	}

	// 层 3：is_default 兜底
	defaults := m.getFilteredAgents(func(a reviewAgent) bool { return a.IsDefault() })
	if len(defaults) > 0 {
		logx.Infof("no stack match for %v; falling back to %d default agents", stacks, len(defaults))
		return defaults
	}

	// 层 4：全部 enabled 兜底（避免无 agent 可跑导致评审失败）
	registered := m.GetAgents()
	logx.Infof("no stack match and no defaults; running all %d registered agents", len(registered))
	return registered
}

// selectForProject 按 persona 白名单选中 agent，并用项目优先级覆盖全局优先级。
// 同一 persona 的冗余实例（路径 A4，如 security、security-2）会一并选中。
func (m *AgentManager) selectForProject(projMap map[string]int) []reviewAgent {
	all := m.GetAgents()
	selected := make([]reviewAgent, 0, len(all))
	for _, a := range all {
		prio, ok := projMap[a.Persona()]
		if !ok {
			continue
		}
		if prio == a.Priority() {
			selected = append(selected, a)
		} else {
			selected = append(selected, priorityOverrideAgent{reviewAgent: a, priority: prio})
		}
	}
	return selected
}

// selectByStack 按 always + stack:* 标签匹配技术栈。stacks 为空时返回 nil。
func (m *AgentManager) selectByStack(stacks []string) []reviewAgent {
	if len(stacks) == 0 {
		return nil
	}
	return m.getFilteredAgents(func(a reviewAgent) bool {
		return tagMatchesStacks(a.Focus(), stacks)
	})
}

// priorityOverrideAgent 嵌入一个 agent 并覆盖其 Priority()（项目优先级覆盖全局）。
type priorityOverrideAgent struct {
	reviewAgent
	priority int
}

func (a priorityOverrideAgent) Priority() int { return a.priority }

// buildEphemeralAgents 构造 ephemeral agent 实例列表（不入库）。factory 未注入时跳过并告警。
func (m *AgentManager) buildEphemeralAgents(ephemeral []EphemeralSpec) []reviewAgent {
	if len(ephemeral) == 0 {
		return nil
	}
	m.mu.RLock()
	factory := m.factory
	m.mu.RUnlock()
	if factory == nil {
		logx.Warnf("ephemeral reviewers requested but no runner factory configured; skipping %d ephemeral", len(ephemeral))
		return nil
	}
	agents := make([]reviewAgent, 0, len(ephemeral))
	for _, spec := range ephemeral {
		name := spec.Name
		if name == "" {
			name = "ephemeral"
		}
		runner := factory.CreateRunner(spec.Background, "")
		agents = append(agents, newOCRAgent(runner, ocrAgentConfig{
			Name:       name,
			Persona:    name,
			Priority:   100, // 临时 agent 优先级较低
			Focus:      []string{"ad-hoc"},
			Background: spec.Background,
		}))
	}
	return agents
}

// runAgents 是执行 agent 的公共流程：拼接 ephemeral → 并行运行 → 聚合。
func (m *AgentManager) runAgents(ctx context.Context, agents []reviewAgent, ephemeral []EphemeralSpec, workspace Workspace, callback func(AgentStatus)) (AggregatedResult, error) {
	agents = append(append([]reviewAgent{}, agents...), m.buildEphemeralAgents(ephemeral)...)
	if len(agents) == 0 {
		return AggregatedResult{}, fmt.Errorf("no agents available for review")
	}

	ctx, cancel := context.WithTimeout(ctx, m.config.TotalTimeout)
	defer cancel()

	logx.Infof("starting review with %d agents (%d ephemeral)", len(agents), len(ephemeral))
	startTime := time.Now()

	agentResults, agentErrors := m.runParallel(ctx, agents, workspace, callback)

	if len(agentErrors) > 0 {
		logx.Warnf("%d agents failed:", len(agentErrors))
		for _, err := range agentErrors {
			logx.Warnf("  - %v", err)
		}
	}

	if len(agentResults) == 0 {
		// PR-09：使用 errors.Join 保留全部子错误的 Unwrap 链，
		// 让上层 errors.Is / errors.As 可以枚举失败原因。
		return AggregatedResult{}, fmt.Errorf("all %d agents failed: %w", len(agentErrors), errors.Join(agentErrors...))
	}

	aggregated := m.aggregator.Aggregate(agentResults)
	aggregated.Duration = time.Since(startTime)

	logx.Infof("review completed in %v with %d findings (confidence: %.2f)",
		aggregated.Duration, len(aggregated.Findings), aggregated.Confidence)

	return aggregated, nil
}

// runParallel runs agents in parallel with concurrency control.
func (m *AgentManager) runParallel(ctx context.Context, agents []reviewAgent, workspace Workspace, callback func(AgentStatus)) ([]AgentResult, []error) {
	var sem chan struct{}
	if m.config.MaxConcurrency > 0 {
		sem = make(chan struct{}, m.config.MaxConcurrency)
	}

	var (
		mu           sync.Mutex
		agentResults []AgentResult
		agentErrors  []error
		wg           sync.WaitGroup
	)

	for _, a := range agents {
		agent := a
		wg.Add(1)
		async.GoSafe(func() {
			defer wg.Done()

			if sem != nil {
				sem <- struct{}{}
				defer func() { <-sem }()
			}

			if callback != nil {
				callback(AgentStatus{
					Name:     agent.Name(),
					Status:   "running",
					Progress: 0,
					Message:  "starting review",
				})
			}

			// 进度心跳：OCR review 是不透明长任务，没有真实完成度信号。
			// 按已耗时用渐近曲线推算一个平滑递增、渐近 95% 的伪进度，
			// 让前端进度条从 0% 平滑爬升，完成回调再置 100%，避免长时间卡 0%。
			progCtx, progCancel := context.WithCancel(context.Background())
			// defer 保证 runSingleAgent panic 时也能取消 progCtx。
			// progCtx 派生自 Background 永不过期，不取消则心跳 goroutine + context 永久泄漏。
			defer progCancel()
			// 用 async.GoSafe 而非裸 go，避免心跳 goroutine 内 panic 直接打挂整个进程。
			async.GoSafe(func() {
				m.emitAgentProgress(progCtx, agent.Name(), callback, time.Now())
			})

			result, err := m.runSingleAgent(ctx, agent, workspace)
			progCancel() // 正常路径立即停止心跳（cancel 幂等，defer 兜底）
			if err != nil {
				logx.Errorf("agent %s failed: %v", agent.Name(), err)
				if callback != nil {
					callback(AgentStatus{
						Name:    agent.Name(),
						Status:  "failed",
						Message: err.Error(),
						Error:   err.Error(),
					})
				}
				mu.Lock()
				agentErrors = append(agentErrors, err)
				mu.Unlock()
				return
			}

			if callback != nil {
				callback(AgentStatus{
					Name:     agent.Name(),
					Status:   "completed",
					Progress: 100,
					Message:  fmt.Sprintf("found %d findings", len(result.Findings)),
					Duration: result.Duration,
				})
			}

			mu.Lock()
			agentResults = append(agentResults, result)
			mu.Unlock()
		})
	}

	wg.Wait()
	return agentResults, agentErrors
}

// runSingleAgent runs a single agent with timeout
func (m *AgentManager) runSingleAgent(ctx context.Context, agent reviewAgent, workspace Workspace) (AgentResult, error) {
	agentTimeout := m.config.AgentTimeout
	if agentTimeout <= 0 {
		agentTimeout = 5 * time.Minute
	}
	agentCtx, agentCancel := context.WithTimeout(ctx, agentTimeout)
	defer agentCancel()

	logx.Infof("starting agent: %s", agent.Name())
	agentStart := time.Now()

	result, err := agent.Review(agentCtx, workspace)
	if err != nil {
		return AgentResult{}, fmt.Errorf("agent %s failed: %w", agent.Name(), err)
	}

	result.Duration = time.Since(agentStart)
	logx.Infof("agent %s completed in %v with %d findings", agent.Name(), result.Duration, len(result.Findings))

	return result, nil
}

// emitAgentProgress 周期性地用渐近曲线推算 agent 进度并回调。
// OCR review 没有真实的完成度信号，因此用「已耗时 → 渐近曲线」推算一个平滑递增、
// 渐近 95% 的伪进度，避免前端长时间卡 0% 再跳 100%。完成回调负责置 100%。
func (m *AgentManager) emitAgentProgress(ctx context.Context, name string, callback func(AgentStatus), start time.Time) {
	if callback == nil {
		return
	}
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			elapsed := time.Since(start)
			callback(AgentStatus{
				Name:     name,
				Status:   "running",
				Progress: estimateAgentProgress(elapsed),
				Message:  fmt.Sprintf("reviewing (%s)", elapsed.Round(time.Second)),
			})
		}
	}
}

// estimateAgentProgress 按经过时间用渐近曲线推算进度（0–95%）。
// 曲线在 ~90s 时约 50%、~4min 时约 88%，渐近 95% 但永不达到 100%（100% 留给完成回调）。
func estimateAgentProgress(elapsed time.Duration) float64 {
	const maxP, tau = 95.0, 90.0
	secs := elapsed.Seconds()
	if secs <= 0 {
		return 0
	}
	return maxP * (1 - math.Exp(-secs/tau))
}
