package agent

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/cloudwego/hertz/pkg/common/hlog"
)

// AgentManager manages multiple review agents
type AgentManager struct {
	agents     []ReviewAgent
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
func (m *AgentManager) RegisterAgent(agent ReviewAgent) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.agents = append(m.agents, agent)
	hlog.Infof("registered agent: %s (priority: %d)", agent.Name(), agent.Priority())
}

// RegisterAgents registers multiple review agents
func (m *AgentManager) RegisterAgents(agents []ReviewAgent) {
	for _, agent := range agents {
		m.RegisterAgent(agent)
	}
}

// GetAgents returns all registered agents
func (m *AgentManager) GetAgents() []ReviewAgent {
	m.mu.RLock()
	defer m.mu.RUnlock()
	agents := make([]ReviewAgent, len(m.agents))
	copy(agents, m.agents)
	return agents
}

// GetAgent returns an agent by name
func (m *AgentManager) GetAgent(name string) (ReviewAgent, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, agent := range m.agents {
		if agent.Name() == name {
			return agent, nil
		}
	}
	return nil, fmt.Errorf("agent not found: %s", name)
}

// RunAgents runs all registered agents in parallel
func (m *AgentManager) RunAgents(ctx context.Context, workspace Workspace) (AggregatedResult, error) {
	return m.RunAgentsWithFilter(ctx, workspace, nil)
}

// RunAgentsWithFilter runs agents matching the filter in parallel
func (m *AgentManager) RunAgentsWithFilter(ctx context.Context, workspace Workspace, filter func(ReviewAgent) bool) (AggregatedResult, error) {
	m.mu.RLock()
	agents := make([]ReviewAgent, len(m.agents))
	copy(agents, m.agents)
	m.mu.RUnlock()

	// Apply filter
	if filter != nil {
		var filtered []ReviewAgent
		for _, agent := range agents {
			if filter(agent) {
				filtered = append(filtered, agent)
			}
		}
		agents = filtered
	}

	if len(agents) == 0 {
		return AggregatedResult{}, fmt.Errorf("no agents available for review")
	}

	// Create context with total timeout
	ctx, cancel := context.WithTimeout(ctx, m.config.TotalTimeout)
	defer cancel()

	hlog.Infof("starting review with %d agents", len(agents))

	// Create channels for results and errors
	results := make(chan AgentResult, len(agents))
	errors := make(chan error, len(agents))

	// Create semaphore for concurrency control
	sem := make(chan struct{}, m.config.MaxConcurrency)

	// Start agents
	var wg sync.WaitGroup
	startTime := time.Now()

	for _, agent := range agents {
		wg.Add(1)
		go func(a ReviewAgent) {
			defer wg.Done()

			// Acquire semaphore
			sem <- struct{}{}
			defer func() { <-sem }()

			// Create agent context with timeout
			agentTimeout := m.config.AgentTimeout
			if agentTimeout <= 0 {
				agentTimeout = 5 * time.Minute
			}
			agentCtx, agentCancel := context.WithTimeout(ctx, agentTimeout)
			defer agentCancel()

			// Run agent
			hlog.Infof("starting agent: %s", a.Name())
			agentStart := time.Now()

			result, err := a.Review(agentCtx, workspace)
			if err != nil {
				hlog.Errorf("agent %s failed: %v", a.Name(), err)
				errors <- fmt.Errorf("agent %s failed: %w", a.Name(), err)
				return
			}

			result.Duration = time.Since(agentStart)
			hlog.Infof("agent %s completed in %v with %d findings", a.Name(), result.Duration, len(result.Findings))

			results <- result
		}(agent)
	}

	// Wait for all agents to complete
	go func() {
		wg.Wait()
		close(results)
		close(errors)
	}()

	// Collect results
	var agentResults []AgentResult
	var agentErrors []error

	for {
		select {
		case result, ok := <-results:
			if !ok {
				results = nil
			} else {
				agentResults = append(agentResults, result)
			}
		case err, ok := <-errors:
			if !ok {
				errors = nil
			} else {
				agentErrors = append(agentErrors, err)
			}
		}

		if results == nil && errors == nil {
			break
		}
	}

	// Log errors
	if len(agentErrors) > 0 {
		hlog.Warnf("%d agents failed:", len(agentErrors))
		for _, err := range agentErrors {
			hlog.Warnf("  - %v", err)
		}
	}

	// Check if we have any results
	if len(agentResults) == 0 {
		return AggregatedResult{}, fmt.Errorf("all agents failed: %v", agentErrors)
	}

	// Aggregate results
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

	// Create context with timeout
	ctx, cancel := context.WithTimeout(ctx, m.config.AgentTimeout)
	defer cancel()

	// Run agent
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
	m.mu.RLock()
	agents := make([]ReviewAgent, len(m.agents))
	copy(agents, m.agents)
	m.mu.RUnlock()

	if len(agents) == 0 {
		return AggregatedResult{}, fmt.Errorf("no agents available for review")
	}

	// Create context with total timeout
	ctx, cancel := context.WithTimeout(ctx, m.config.TotalTimeout)
	defer cancel()

	hlog.Infof("starting review with %d agents", len(agents))

	// Create channels for results and errors
	results := make(chan AgentResult, len(agents))
	errors := make(chan error, len(agents))

	// Create semaphore for concurrency control
	sem := make(chan struct{}, m.config.MaxConcurrency)

	// Start agents
	var wg sync.WaitGroup
	startTime := time.Now()

	for _, agent := range agents {
		wg.Add(1)
		go func(a ReviewAgent) {
			defer wg.Done()

			// Acquire semaphore
			sem <- struct{}{}
			defer func() { <-sem }()

			// Create agent context with timeout
			agentTimeout := m.config.AgentTimeout
			if agentTimeout <= 0 {
				agentTimeout = 5 * time.Minute
			}
			agentCtx, agentCancel := context.WithTimeout(ctx, agentTimeout)
			defer agentCancel()

			// Report status
			callback(AgentStatus{
				Name:     a.Name(),
				Status:   "running",
				Progress: 0,
				Message:  "starting review",
			})

			// Run agent
			agentStart := time.Now()
			result, err := a.Review(agentCtx, workspace)
			if err != nil {
				hlog.Errorf("agent %s failed: %v", a.Name(), err)
				callback(AgentStatus{
					Name:     a.Name(),
					Status:   "failed",
					Progress: 0,
					Message:  err.Error(),
					Duration: time.Since(agentStart),
					Error:    err.Error(),
				})
				errors <- fmt.Errorf("agent %s failed: %w", a.Name(), err)
				return
			}

			result.Duration = time.Since(agentStart)
			hlog.Infof("agent %s completed in %v with %d findings", a.Name(), result.Duration, len(result.Findings))

			// Report status
			callback(AgentStatus{
				Name:     a.Name(),
				Status:   "completed",
				Progress: 100,
				Message:  fmt.Sprintf("found %d findings", len(result.Findings)),
				Duration: result.Duration,
			})

			results <- result
		}(agent)
	}

	// Wait for all agents to complete
	go func() {
		wg.Wait()
		close(results)
		close(errors)
	}()

	// Collect results
	var agentResults []AgentResult
	var agentErrors []error

	for {
		select {
		case result, ok := <-results:
			if !ok {
				results = nil
			} else {
				agentResults = append(agentResults, result)
			}
		case err, ok := <-errors:
			if !ok {
				errors = nil
			} else {
				agentErrors = append(agentErrors, err)
			}
		}

		if results == nil && errors == nil {
			break
		}
	}

	// Log errors
	if len(agentErrors) > 0 {
		hlog.Warnf("%d agents failed:", len(agentErrors))
		for _, err := range agentErrors {
			hlog.Warnf("  - %v", err)
		}
	}

	// Check if we have any results
	if len(agentResults) == 0 {
		return AggregatedResult{}, fmt.Errorf("all agents failed: %v", agentErrors)
	}

	// Aggregate results
	aggregated := m.aggregator.Aggregate(agentResults)
	aggregated.Duration = time.Since(startTime)

	hlog.Infof("review completed in %v with %d findings (confidence: %.2f)",
		aggregated.Duration, len(aggregated.Findings), aggregated.Confidence)

	return aggregated, nil
}
