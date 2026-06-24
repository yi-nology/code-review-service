package agent

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/oekit/concurrency/async"
	"github.com/oekit/pkg/slicex"
	"github.com/oekit/logx"
)

// AgentManager manages multiple review agents
type AgentManager struct {
	agents     []reviewAgent
	aggregator ResultAggregator
	config     ManagerConfig
	factory    reviewRunnerFactory // 用于构造 ephemeral agent 的 runner（阶段 5）
	mu         sync.RWMutex
}

// SetFactory 注入 runner 工厂，使 AgentManager 具备构造临时（ephemeral）agent 的能力。
func (m *AgentManager) SetFactory(factory reviewRunnerFactory) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.factory = factory
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

// RegisterAgent registers a review agent
func (m *AgentManager) RegisterAgent(agent reviewAgent) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.agents = append(m.agents, agent)
	logx.Infof("registered agent: %s (priority: %d)", agent.Name(), agent.Priority())
}

// RegisterAgents registers multiple review agents
func (m *AgentManager) RegisterAgents(agents []reviewAgent) {
	for _, agent := range agents {
		m.RegisterAgent(agent)
	}
}

// GetAgents returns all registered agents
func (m *AgentManager) GetAgents() []reviewAgent {
	m.mu.RLock()
	defer m.mu.RUnlock()
	agents := make([]reviewAgent, len(m.agents))
	copy(agents, m.agents)
	return agents
}

// GetAgent returns an agent by name
func (m *AgentManager) GetAgent(name string) (reviewAgent, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, agent := range m.agents {
		if agent.Name() == name {
			return agent, nil
		}
	}
	return nil, fmt.Errorf("agent not found: %s", name)
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

// RunAgents runs all registered agents in parallel
func (m *AgentManager) RunAgents(ctx context.Context, workspace Workspace) (AggregatedResult, error) {
	return m.RunAgentsWithCallback(ctx, workspace, nil)
}

// RunAgentsWithFilter runs agents matching the filter in parallel
func (m *AgentManager) RunAgentsWithFilter(ctx context.Context, workspace Workspace, filter func(reviewAgent) bool) (AggregatedResult, error) {
	agents := m.getFilteredAgents(filter)
	if len(agents) == 0 {
		return AggregatedResult{}, fmt.Errorf("no agents available for review")
	}

	ctx, cancel := context.WithTimeout(ctx, m.config.TotalTimeout)
	defer cancel()

	logx.Infof("starting review with %d agents", len(agents))
	startTime := time.Now()

	agentResults, agentErrors := m.runParallel(ctx, agents, workspace, nil)

	if len(agentErrors) > 0 {
		logx.Warnf("%d agents failed:", len(agentErrors))
		for _, err := range agentErrors {
			logx.Warnf("  - %v", err)
		}
	}

	if len(agentResults) == 0 {
		return AggregatedResult{}, fmt.Errorf("all agents failed: %v", agentErrors)
	}

	aggregated := m.aggregator.Aggregate(agentResults)
	aggregated.Duration = time.Since(startTime)

	logx.Infof("review completed in %v with %d findings (confidence: %.2f)",
		aggregated.Duration, len(aggregated.Findings), aggregated.Confidence)

	return aggregated, nil
}

// RunAgent runs a single agent by name
func (m *AgentManager) RunAgent(ctx context.Context, agentName string, workspace Workspace) (AgentResult, error) {
	agent, err := m.GetAgent(agentName)
	if err != nil {
		return AgentResult{}, err
	}

	ctx, cancel := context.WithTimeout(ctx, m.config.AgentTimeout)
	defer cancel()

	startTime := time.Now()
	result, err := agent.Review(ctx, workspace)
	if err != nil {
		return AgentResult{}, fmt.Errorf("agent %s failed: %w", agentName, err)
	}

	result.Duration = time.Since(startTime)
	return result, nil
}

// AgentStatus represents the status of an agent
type AgentStatus struct {
	Name     string        `json:"name"`
	Status   string        `json:"status"`
	Progress float64       `json:"progress"`
	Message  string        `json:"message"`
	Duration time.Duration `json:"duration"`
	Error    string        `json:"error,omitempty"`
}

// RunAgentsWithCallback runs agents with progress callbacks
func (m *AgentManager) RunAgentsWithCallback(ctx context.Context, workspace Workspace, callback func(status AgentStatus)) (AggregatedResult, error) {
	return m.RunAgentsWithEphemeral(ctx, workspace, nil, callback)
}

// RunAgentsWithEphemeral 运行已注册 agent + 临时（ephemeral）agent（阶段 5）。
// ephemeral agent 仅存在于本次调用，不入库、不持久化。
func (m *AgentManager) RunAgentsWithEphemeral(ctx context.Context, workspace Workspace, ephemeral []EphemeralSpec, callback func(status AgentStatus)) (AggregatedResult, error) {
	agents := m.getFilteredAgents(nil)
	if len(agents) == 0 && len(ephemeral) == 0 {
		return AggregatedResult{}, fmt.Errorf("no agents available for review")
	}

	// 构造 ephemeral agent（需要 factory）
	if len(ephemeral) > 0 {
		m.mu.RLock()
		factory := m.factory
		m.mu.RUnlock()
		if factory == nil {
			logx.Warnf("ephemeral reviewers requested but no runner factory configured; skipping %d ephemeral", len(ephemeral))
		} else {
			for _, spec := range ephemeral {
				name := spec.Name
				if name == "" {
					name = "ephemeral"
				}
				runner := factory.CreateRunner(spec.Background, "")
				agents = append(agents, newOCRAgent(runner, ocrAgentConfig{
					Name:     name,
					Persona:  name,
					Priority: 100, // 临时 agent 优先级较低
					Focus:    []string{"ad-hoc"},
				}))
			}
		}
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
		return AggregatedResult{}, fmt.Errorf("all agents failed: %v", agentErrors)
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

			result, err := m.runSingleAgent(ctx, agent, workspace)
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
