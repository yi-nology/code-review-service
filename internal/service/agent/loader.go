package agent

import (
	"context"
	"fmt"
	"time"

	"github.com/yi-nology/code-review-service/internal/repository/postgres/models"
	"github.com/oekit/logx"
)

// reviewRunnerFactory creates review runners with different configurations
type reviewRunnerFactory interface {
	// CreateRunner creates a runner with the given background and rule content (路径 A1)
	CreateRunner(background, ruleContent string) reviewRunner
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
		logx.Info("no agents found in database, using default single agent")
		return nil
	}

	loaded := 0
	for _, agentModel := range agents {
		if !agentModel.Enabled {
			logx.Infof("skipping disabled agent: %s", agentModel.Name)
			continue
		}

		// 冗余次数（路径 A4）：默认 1，<1 视为 1
		redundancy := agentModel.Redundancy
		if redundancy < 1 {
			redundancy = 1
		}

		for i := 1; i <= redundancy; i++ {
			// 每个 persona 实例共享同一 runner（相同 background + rule）
			runnerInstance := l.factory.CreateRunner(agentModel.Background, agentModel.RuleFile)

			// 实例名：冗余>1 时加后缀，便于聚合时区分；persona 基名保持 agentModel.Name
			instanceName := agentModel.Name
			if redundancy > 1 {
				instanceName = fmt.Sprintf("%s-%d", agentModel.Name, i)
			}

			agentConfig := ocrAgentConfig{
				Name:      instanceName,
				Persona:   agentModel.Name,
				Priority:  agentModel.Priority,
				Focus:     extractTags(agentModel.Tags),
				Timeout:   l.timeout,
				IsDefault: agentModel.IsDefault,
			}

			agentInstance := newOCRAgent(runnerInstance, agentConfig)
			l.agentManager.RegisterAgent(agentInstance)
			loaded++
		}

		logx.Infof("loaded agent from DB: %s (priority: %d, background: %d chars, rule: %d chars, redundancy: %d)",
			agentModel.Name, agentModel.Priority, len(agentModel.Background), len(agentModel.RuleFile), redundancy)
	}

	logx.Infof("loaded %d agents from database", loaded)
	return nil
}

// LoadDefaultAgent loads a single default agent with the given background
func (l *AgentLoader) LoadDefaultAgent(background string) {
	runner := l.factory.CreateRunner(background, "")

	agentConfig := ocrAgentConfig{
		Name:      "default",
		Priority:  l.priority,
		Focus:     []string{"code_quality", "best_practices", "potential_bugs"},
		Timeout:   l.timeout,
		IsDefault: true,
	}

	agent := newOCRAgent(runner, agentConfig)
	l.agentManager.RegisterAgent(agent)

	logx.Infof("loaded default agent with background: %d chars", len(background))
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
