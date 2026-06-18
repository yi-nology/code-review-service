package agent

import (
	"context"
	"time"

	"github.com/yi-nology/code-review-service/internal/service/types"
)

// reviewAgent defines the interface for review agents (internal only)
type reviewAgent interface {
	Name() string
	Focus() []string
	Review(ctx context.Context, workspace Workspace) (AgentResult, error)
	Priority() int
}

// Workspace contains the review context
type Workspace struct {
	Dir          string
	BaseSHA      string
	HeadSHA      string
	Diff         string
	Requirements *Requirements
	Context      map[string]interface{}
}

// AgentResult contains the agent's review results
type AgentResult struct {
	AgentName  string        `json:"agent_name"`
	Findings   []Finding     `json:"findings"`
	Summary    string        `json:"summary"`
	Confidence float64       `json:"confidence"`
	Duration   time.Duration `json:"duration"`
	TokensUsed int64         `json:"tokens_used"`
	Warnings   []warning     `json:"warnings"`
}

// Finding is an alias for the canonical types.Finding
type Finding = types.Finding

// Warning represents a review warning
type warning struct {
	Type    string `json:"type"`
	Message string `json:"message"`
	Path    string `json:"path"`
}

// Requirements contains review requirements
type Requirements struct {
	Inline  []string          `json:"inline"`
	Files   []string          `json:"files"`
	URLs    []string          `json:"urls"`
	Context map[string]string `json:"context"`
}

// agentConfig contains agent configuration
type agentConfig struct {
	Name     string   `yaml:"name"`
	Count    int      `yaml:"count"`
	Focus    []string `yaml:"focus"`
	Enabled  bool     `yaml:"enabled"`
	Priority int      `yaml:"priority"`
	Timeout  string   `yaml:"timeout"`
}

// AggregatedResult contains aggregated results from multiple agents
type AggregatedResult struct {
	Findings     []Finding     `json:"findings"`
	Summary      string        `json:"summary"`
	AgentResults []AgentResult `json:"agent_results"`
	Confidence   float64       `json:"confidence"`
	Duration     time.Duration `json:"duration"`
}

// ResultAggregator defines the interface for result aggregation
type ResultAggregator interface {
	// Aggregate combines results from multiple agents
	Aggregate(results []AgentResult) AggregatedResult
}
