package agent

import (
	"context"
	"time"

	"github.com/yi-nology/code-review-service/internal/service/types"
)

// ReviewAgent defines the interface for review agents
type ReviewAgent interface {
	// Name returns the agent name
	Name() string

	// Focus returns the agent's focus areas
	Focus() []string

	// Review performs the code review
	Review(ctx context.Context, workspace Workspace) (AgentResult, error)

	// Priority returns the agent priority (lower = higher priority)
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
	Warnings   []Warning     `json:"warnings"`
}

// Finding is an alias for the canonical types.Finding
type Finding = types.Finding

// Warning represents a review warning
type Warning struct {
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

// AgentConfig contains agent configuration
type AgentConfig struct {
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
