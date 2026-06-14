package agent

import (
	"fmt"
	"time"
)

// AgentTeamConfig defines the configuration for a team of agents
type AgentTeamConfig struct {
	// DefaultTeam is the default team composition
	DefaultTeam []AgentConfig `yaml:"default_team"`

	// CustomAgents defines custom agent configurations
	CustomAgents []AgentConfig `yaml:"custom_agents"`

	// MaxConcurrency is the maximum number of agents to run in parallel
	MaxConcurrency int `yaml:"max_concurrency"`

	// AgentTimeout is the timeout for each agent
	AgentTimeout time.Duration `yaml:"agent_timeout"`

	// TotalTimeout is the total timeout for all agents
	TotalTimeout time.Duration `yaml:"total_timeout"`
}

// DefaultAgentTeamConfig returns default agent team configuration
func DefaultAgentTeamConfig() AgentTeamConfig {
	return AgentTeamConfig{
		DefaultTeam: []AgentConfig{
			{
				Name:     "principal",
				Count:    2,
				Focus:    []string{"architecture", "design_patterns", "maintainability"},
				Enabled:  true,
				Priority: 1,
			},
			{
				Name:     "quality",
				Count:    2,
				Focus:    []string{"code_style", "readability", "best_practices"},
				Enabled:  true,
				Priority: 2,
			},
		},
		CustomAgents: []AgentConfig{
			{
				Name:     "security",
				Count:    1,
				Focus:    []string{"security", "vulnerabilities", "authentication"},
				Enabled:  false,
				Priority: 3,
			},
			{
				Name:     "testing",
				Count:    1,
				Focus:    []string{"testing", "coverage", "test_quality"},
				Enabled:  false,
				Priority: 4,
			},
			{
				Name:     "performance",
				Count:    1,
				Focus:    []string{"performance", "optimization", "scalability"},
				Enabled:  false,
				Priority: 5,
			},
		},
		MaxConcurrency: 4,
		AgentTimeout:   5 * time.Minute,
		TotalTimeout:   30 * time.Minute,
	}
}

// AgentConfigValidator validates agent configurations
type AgentConfigValidator struct{}

// NewAgentConfigValidator creates a new agent config validator
func NewAgentConfigValidator() *AgentConfigValidator {
	return &AgentConfigValidator{}
}

// Validate validates an agent configuration
func (v *AgentConfigValidator) Validate(config AgentConfig) error {
	if config.Name == "" {
		return fmt.Errorf("agent name is required")
	}

	if config.Count < 0 {
		return fmt.Errorf("agent count must be non-negative for %s", config.Name)
	}

	if config.Count > 10 {
		return fmt.Errorf("agent count cannot exceed 10 for %s", config.Name)
	}

	if config.Priority < 0 {
		return fmt.Errorf("agent priority must be non-negative for %s", config.Name)
	}

	return nil
}

// ValidateTeamConfig validates a team configuration
func (v *AgentConfigValidator) ValidateTeamConfig(config AgentTeamConfig) error {
	// Validate default team
	for _, agentConfig := range config.DefaultTeam {
		if err := v.Validate(agentConfig); err != nil {
			return fmt.Errorf("invalid default team config: %w", err)
		}
	}

	// Validate custom agents
	for _, agentConfig := range config.CustomAgents {
		if err := v.Validate(agentConfig); err != nil {
			return fmt.Errorf("invalid custom agent config: %w", err)
		}
	}

	// Validate concurrency
	if config.MaxConcurrency < 1 {
		return fmt.Errorf("max concurrency must be at least 1")
	}

	if config.MaxConcurrency > 20 {
		return fmt.Errorf("max concurrency cannot exceed 20")
	}

	// Validate timeouts
	if config.AgentTimeout <= 0 {
		return fmt.Errorf("agent timeout must be positive")
	}

	if config.TotalTimeout <= 0 {
		return fmt.Errorf("total timeout must be positive")
	}

	if config.AgentTimeout > config.TotalTimeout {
		return fmt.Errorf("agent timeout cannot exceed total timeout")
	}

	return nil
}

// GetEnabledAgents returns the list of enabled agent configurations
func GetEnabledAgents(config AgentTeamConfig) []AgentConfig {
	var enabled []AgentConfig

	// Add default team agents
	for _, agentConfig := range config.DefaultTeam {
		if agentConfig.Enabled {
			enabled = append(enabled, agentConfig)
		}
	}

	// Add custom agents
	for _, agentConfig := range config.CustomAgents {
		if agentConfig.Enabled {
			enabled = append(enabled, agentConfig)
		}
	}

	return enabled
}

// GetAgentConfigByName returns an agent configuration by name
func GetAgentConfigByName(config AgentTeamConfig, name string) (*AgentConfig, error) {
	// Search in default team
	for _, agentConfig := range config.DefaultTeam {
		if agentConfig.Name == name {
			return &agentConfig, nil
		}
	}

	// Search in custom agents
	for _, agentConfig := range config.CustomAgents {
		if agentConfig.Name == name {
			return &agentConfig, nil
		}
	}

	return nil, fmt.Errorf("agent config not found: %s", name)
}

// MergeAgentConfigs merges multiple agent configurations
func MergeAgentConfigs(configs ...AgentConfig) AgentConfig {
	if len(configs) == 0 {
		return AgentConfig{}
	}

	merged := configs[0]
	for _, config := range configs[1:] {
		if config.Name != "" {
			merged.Name = config.Name
		}
		if config.Count > 0 {
			merged.Count = config.Count
		}
		if len(config.Focus) > 0 {
			merged.Focus = config.Focus
		}
		if config.Priority > 0 {
			merged.Priority = config.Priority
		}
		if config.Timeout != "" {
			merged.Timeout = config.Timeout
		}
	}

	return merged
}
