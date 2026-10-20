package agent

import (
	"context"
	"time"

	types "github.com/yi-nology/code-review-service/internal/domain/finding"
)

// reviewAgent defines the interface for review agents (internal only)
type reviewAgent interface {
	Name() string
	Persona() string
	Focus() []string
	Review(ctx context.Context, workspace Workspace) (AgentResult, error)
	Priority() int
	IsDefault() bool
}

// Workspace contains the review context
type Workspace struct {
	Dir     string
	BaseSHA string
	HeadSHA string
	Diff    string
	// RulePath 是合并后的规则文件路径（项目+规则包+全局规则），透传给 OCR 的 --rule。
	// 由 review/executor 在解析规则后注入，覆盖 agent 自身的 cfg.RulePath。
	RulePath     string
	Requirements *Requirements
	Context      map[string]interface{}
	// MemoryContext 是召回的跨会话长期记忆（底座 agent-memory）。
	// 注入进 OCR 的 --background，与 persona 背景拼接后一起给模型。
	MemoryContext string
}

// AgentResult contains the agent's review results
type AgentResult struct {
	AgentName  string        `json:"agent_name"`
	Persona    string        `json:"persona"`
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
