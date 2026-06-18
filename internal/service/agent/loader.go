package agent

import (
	"context"

	"time"

	"github.com/yi-nology/code-review-service/internal/repository/postgres/models"
	"github.com/cloudwego/hertz/pkg/common/hlog"
)

// reviewRunnerFactory creates review runners with different configurations
type reviewRunnerFactory interface {
	// CreateRunner creates a runner with the given background
	CreateRunner(background string) reviewRunner
}

// AgentLoader loads agents from database
type AgentLoader struct {
	agentManager *AgentManager
	factory      reviewRunnerFactory
	priority     int
	timeout      time.Duration
}

// NewAgentLoader creates a new agent loader
func NewAgentLoader(agentManager *AgentManager, factory reviewRunnerFactory, timeout time.Duration) *AgentLoader {
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
		agentConfig := ocrAgentConfig{
			Name:     agentModel.Name,
			Priority: agentModel.Priority,
			Focus:    extractTags(agentModel.Tags),
			Timeout:  l.timeout,
		}

		agent := newOCRAgent(runner, agentConfig)
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

	agentConfig := ocrAgentConfig{
		Name:     "default",
		Priority: l.priority,
		Focus:    []string{"code_quality", "best_practices", "potential_bugs"},
		Timeout:  l.timeout,
	}

	agent := newOCRAgent(runner, agentConfig)
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
