package agent

import (
	"context"
	"fmt"
	"time"

	"github.com/yi-nology/code-review-service/internal/repository/postgres/models"
	"github.com/cloudwego/hertz/pkg/common/hlog"
)

// ReviewRunnerFactory creates review runners with different configurations
type ReviewRunnerFactory interface {
	// CreateRunner creates a runner with the given background
	CreateRunner(background string) ReviewRunner
}

// AgentLoader loads agents from database
type AgentLoader struct {
	agentManager *AgentManager
	factory      ReviewRunnerFactory
	priority     int
	timeout      time.Duration
}

// NewAgentLoader creates a new agent loader
func NewAgentLoader(agentManager *AgentManager, factory ReviewRunnerFactory, timeout time.Duration) *AgentLoader {
	return &AgentLoader{
		agentManager: agentManager,
		factory:      factory,
		priority:     10,
		timeout:      timeout,
	}
}

// LoadFromDB loads agents from database and registers them
func (l *AgentLoader) LoadFromDB(ctx context.Context, agents []models.ReviewerAgent) error {
	if len(agents) == 0 {
		hlog.Info("no agents found in database, using default single agent")
		return nil
	}

	loaded := 0
	for _, agentModel := range agents {
		if !agentModel.Enabled {
			hlog.Infof("skipping disabled agent: %s", agentModel.Name)
			continue
		}

		// Create runner with agent-specific background
		runner := l.factory.CreateRunner(agentModel.Background)

		// Create agent config
		agentConfig := OCRAgentConfig{
			Name:     agentModel.Name,
			Priority: agentModel.Priority,
			Focus:    extractTags(agentModel.Tags),
			Timeout:  l.timeout,
		}

		// Create and register agent
		agent := NewOCRAgent(runner, agentConfig)
		l.agentManager.RegisterAgent(agent)
		loaded++

		hlog.Infof("loaded agent from DB: %s (priority: %d, background: %d chars)",
			agentModel.Name, agentModel.Priority, len(agentModel.Background))
	}

	hlog.Infof("loaded %d agents from database", loaded)
	return nil
}

// LoadDefaultAgent loads a single default agent with the given background
func (l *AgentLoader) LoadDefaultAgent(background string) {
	runner := l.factory.CreateRunner(background)

	agentConfig := OCRAgentConfig{
		Name:     "default",
		Priority: l.priority,
		Focus:    []string{"code_quality", "best_practices", "potential_bugs"},
		Timeout:  l.timeout,
	}

	agent := NewOCRAgent(runner, agentConfig)
	l.agentManager.RegisterAgent(agent)

	hlog.Infof("loaded default agent with background: %d chars", len(background))
}

// extractTags extracts tags from JSON
func extractTags(tags interface{}) []string {
	// Default tags if none specified
	defaultTags := []string{"code_quality", "best_practices"}

	// Try to parse tags from interface
	if tags == nil {
		return defaultTags
	}

	// If tags is a slice, convert to string slice
	if tagSlice, ok := tags.([]interface{}); ok {
		result := make([]string, 0, len(tagSlice))
		for _, tag := range tagSlice {
			if str, ok := tag.(string); ok {
				result = append(result, str)
			}
		}
		if len(result) > 0 {
			return result
		}
	}

	return defaultTags
}

// CreateAgentFromModel creates an agent from a database model
func CreateAgentFromModel(agentModel models.ReviewerAgent, factory ReviewRunnerFactory, timeout time.Duration) (ReviewAgent, error) {
	if !agentModel.Enabled {
		return nil, fmt.Errorf("agent %s is disabled", agentModel.Name)
	}

	// Create runner with agent-specific background
	runner := factory.CreateRunner(agentModel.Background)

	// Create agent config
	agentConfig := OCRAgentConfig{
		Name:     agentModel.Name,
		Priority: agentModel.Priority,
		Focus:    extractTags(agentModel.Tags),
		Timeout:  timeout,
	}

	return NewOCRAgent(runner, agentConfig), nil
}

// CreateProjectAgentManager creates an agent manager for a specific project
func (l *AgentLoader) CreateProjectAgentManager(projectAgents []models.ProjectAgent, allAgents []models.ReviewerAgent) *AgentManager {
	manager := NewAgentManager(l.agentManager.config, l.agentManager.aggregator)

	// Build agent map
	agentMap := make(map[uint]models.ReviewerAgent)
	for _, a := range allAgents {
		agentMap[a.ID] = a
	}

	// If no project config, use all enabled agents
	if len(projectAgents) == 0 {
		hlog.Info("no project agent config, using all enabled agents")
		for _, a := range allAgents {
			if a.Enabled {
				agent, err := CreateAgentFromModel(a, l.factory, l.timeout)
				if err != nil {
					hlog.Warnf("failed to create agent %s: %v", a.Name, err)
					continue
				}
				manager.RegisterAgent(agent)
			}
		}
		return manager
	}

	// Use project-specific agents
	for _, pa := range projectAgents {
		if !pa.Enabled {
			continue
		}

		agentModel, ok := agentMap[pa.AgentID]
		if !ok {
			hlog.Warnf("agent %d not found for project", pa.AgentID)
			continue
		}

		if !agentModel.Enabled {
			hlog.Infof("agent %s is globally disabled, skipping for project", agentModel.Name)
			continue
		}

		// Create runner with agent-specific background
		runner := l.factory.CreateRunner(agentModel.Background)

		// Create agent config with project-specific priority
		agentConfig := OCRAgentConfig{
			Name:     agentModel.Name,
			Priority: pa.Priority, // Use project-specific priority
			Focus:    extractTags(agentModel.Tags),
			Timeout:  l.timeout,
		}

		agent := NewOCRAgent(runner, agentConfig)
		manager.RegisterAgent(agent)

		hlog.Infof("loaded project agent: %s (priority: %d)", agentModel.Name, pa.Priority)
	}

	return manager
}
