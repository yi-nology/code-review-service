package agent

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/oekit/pkg/slicex"
	"github.com/oekit/pkg"
	"github.com/cloudwego/hertz/pkg/common/hlog"
)

// AgentManager manages multiple review agents
type AgentManager struct {
	agents     []reviewAgent
	aggregator ResultAggregator
	config     ManagerConfig
	mu         sync.RWMutex
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
		AgentTimeout:   5 * time.Minute,
		TotalTimeout:   30 * time.Minute,
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
	hlog.Infof("registered agent: %s (priority: %d)", agent.Name(), agent.Priority())
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

	hlog.Infof("starting review with %d agents", len(agents))
	startTime := time.Now()

	// 使用 pipeline.ParallelFlow 简化并发控制
	parallel := pipeline.NewParallel(m.config.MaxConcurrency)

	var (
		mu           sync.Mutex
		agentResults []AgentResult
		agentErrors  []error
	)

	for _, a := range agents {
		agent := a
		parallel.Add(func(ctx context.Context) error {
			result, err := m.runSingleAgent(ctx, agent, workspace)
			if err != nil {
				mu.Lock()
				agentErrors = append(agentErrors, err)
				mu.Unlock()
				return nil // 不中断其他 agent
			}
			mu.Lock()
			agentResults = append(agentResults, result)
			mu.Unlock()
			return nil
		})
	}

	// 并发执行所有 agent
	_ = parallel.Run(ctx)

	// 记录错误
	if len(agentErrors) > 0 {
		hlog.Warnf("%d agents failed:", len(agentErrors))
		for _, err := range agentErrors {
			hlog.Warnf("  - %v", err)
		}
	}

	// 检查是否有结果
	if len(agentResults) == 0 {
		return AggregatedResult{}, fmt.Errorf("all agents failed: %v", agentErrors)
	}

	// 聚合结果
	aggregated := m.aggregator.Aggregate(agentResults)
	aggregated.Duration = time.Since(startTime)

	hlog.Infof("review completed in %v with %d findings (confidence: %.2f)",
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
	Name      string        `json:"name"`
	Status    string        `json:"status"`
	Progress  float64       `json:"progress"`
	Message   string        `json:"message"`
	Duration  time.Duration `json:"duration"`
	Error     string        `json:"error,omitempty"`
}

// RunAgentsWithCallback runs agents with progress callbacks
func (m *AgentManager) RunAgentsWithCallback(ctx context.Context, workspace Workspace, callback func(status AgentStatus)) (AggregatedResult, error) {
	agents := m.getFilteredAgents(nil)
	if len(agents) == 0 {
		return AggregatedResult{}, fmt.Errorf("no agents available for review")
	}

	ctx, cancel := context.WithTimeout(ctx, m.config.TotalTimeout)
	defer cancel()

	hlog.Infof("starting review with %d agents", len(agents))
	startTime := time.Now()

	// 使用 pipeline.ParallelFlow 简化并发控制
	parallel := pipeline.NewParallel(m.config.MaxConcurrency)

	var (
		mu           sync.Mutex
		agentResults []AgentResult
		agentErrors  []error
	)

	for _, a := range agents {
		agent := a
		parallel.Add(func(ctx context.Context) error {
			// 报告状态
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
				hlog.Errorf("agent %s failed: %v", agent.Name(), err)
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
				return nil // 不中断其他 agent
			}

			// 报告完成状态
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
			return nil
		})
	}

	// 并发执行所有 agent
	_ = parallel.Run(ctx)

	// 记录错误
	if len(agentErrors) > 0 {
		hlog.Warnf("%d agents failed:", len(agentErrors))
		for _, err := range agentErrors {
			hlog.Warnf("  - %v", err)
		}
	}

	// 检查是否有结果
	if len(agentResults) == 0 {
		return AggregatedResult{}, fmt.Errorf("all agents failed: %v", agentErrors)
	}

	// 聚合结果
	aggregated := m.aggregator.Aggregate(agentResults)
	aggregated.Duration = time.Since(startTime)

	hlog.Infof("review completed in %v with %d findings (confidence: %.2f)",
		aggregated.Duration, len(aggregated.Findings), aggregated.Confidence)

	return aggregated, nil
}

// runSingleAgent runs a single agent with timeout
func (m *AgentManager) runSingleAgent(ctx context.Context, agent reviewAgent, workspace Workspace) (AgentResult, error) {
	agentTimeout := m.config.AgentTimeout
	if agentTimeout <= 0 {
		agentTimeout = 5 * time.Minute
	}
	agentCtx, agentCancel := context.WithTimeout(ctx, agentTimeout)
	defer agentCancel()

	hlog.Infof("starting agent: %s", agent.Name())
	agentStart := time.Now()

	result, err := agent.Review(agentCtx, workspace)
	if err != nil {
		return AgentResult{}, fmt.Errorf("agent %s failed: %w", agent.Name(), err)
	}

	result.Duration = time.Since(agentStart)
	hlog.Infof("agent %s completed in %v with %d findings", agent.Name(), result.Duration, len(result.Findings))

	return result, nil
}
