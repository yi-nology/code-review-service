package eino

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/yi-nology/code-review-service/internal/service/agent"
	"github.com/yi-nology/code-review-service/internal/service/review/domain"

	"github.com/cloudwego/eino/compose"
)

// AgentReviewInput is the input for multi-agent review
type AgentReviewInput struct {
	ReviewID  string           `json:"review_id"`
	Request   domain.Request   `json:"request"`
	Workspace domain.Workspace `json:"workspace"`
}

// AgentReviewOutput is the output from multi-agent review
type AgentReviewOutput struct {
	Findings         []domain.Finding  `json:"findings"`
	AgentResults     []AgentResult     `json:"agent_results"`
	AggregatedResult *AggregatedResult `json:"aggregated_result"`
}

// AgentResult represents a single agent's review result
type AgentResult struct {
	AgentName string           `json:"agent_name"`
	Findings  []domain.Finding `json:"findings"`
	Error     string           `json:"error,omitempty"`
}

// AggregatedResult represents the aggregated result from all agents
type AggregatedResult struct {
	TotalFindings  int              `json:"total_findings"`
	UniqueFindings int              `json:"unique_findings"`
	DuplicateCount int              `json:"duplicate_count"`
	Findings       []domain.Finding `json:"findings"`
}

// BuildMultiAgentWorkflow creates an eino compose.Chain workflow for parallel agent execution.
// This replaces the serial multi-agent execution in the monolith.
//
// The workflow uses AgentManager.RunAgentsForProject for parallel execution,
// then aggregates and deduplicates findings.
func BuildMultiAgentWorkflow(agentManager *agent.AgentManager) (*compose.Chain[*AgentReviewInput, *AgentReviewOutput], error) {
	chain := compose.NewChain[*AgentReviewInput, *AgentReviewOutput]()

	// Node 1: Resolve agents and prepare input
	resolveNode := compose.InvokableLambda(func(ctx context.Context, input *AgentReviewInput) (*AgentReviewInput, error) {
		// Agent resolution happens here if needed
		return input, nil
	})
	chain.AppendLambda(resolveNode, compose.WithNodeName("resolve_agents"))

	// Node 2: Run agents in parallel using AgentManager
	reviewNode := compose.InvokableLambda(func(ctx context.Context, input *AgentReviewInput) ([]AgentResult, error) {
		if agentManager == nil {
			return nil, fmt.Errorf("agent manager not configured")
		}

		// Get available agents
		agents := agentManager.GetAgents()
		if len(agents) == 0 {
			return nil, fmt.Errorf("no agents available")
		}

		// Run agents in parallel using AgentManager
		agentWorkspace := agent.Workspace{
			Dir:     input.Workspace.Dir,
			BaseSHA: input.Workspace.BaseSHA,
			HeadSHA: input.Workspace.HeadSHA,
			Diff:    input.Workspace.Diff,
		}

		// RunAgentsForProject expects (ctx, workspace, provider, repo, ephemeral, callback)
		aggregatedResult, err := agentManager.RunAgentsForProject(
			ctx,
			agentWorkspace,
			input.Request.Provider,
			input.Request.Repo,
			nil, // no ephemeral agents
			nil, // no callback
		)
		if err != nil {
			return nil, fmt.Errorf("run agents failed: %w", err)
		}

		// Convert to AgentResult slice
		results := make([]AgentResult, 0, len(aggregatedResult.AgentResults))
		for _, ar := range aggregatedResult.AgentResults {
			result := AgentResult{
				AgentName: ar.AgentName,
			}
			for _, f := range ar.Findings {
				result.Findings = append(result.Findings, domain.Finding{
					Path:       f.Path,
					Line:       f.Line,
					Severity:   f.Severity,
					Title:      f.Title,
					Body:       f.Body,
					Category:   f.Category,
					Suggestion: f.Suggestion,
				})
			}
			results = append(results, result)
		}

		return results, nil
	})
	chain.AppendLambda(reviewNode, compose.WithNodeName("parallel_reviews"))

	// Node 3: Aggregate findings from all agents
	aggregateNode := compose.InvokableLambda(func(ctx context.Context, agentResults []AgentResult) (*AgentReviewOutput, error) {
		// Collect all findings
		var allFindings []domain.Finding
		for _, result := range agentResults {
			allFindings = append(allFindings, result.Findings...)
		}

		// Deduplicate findings
		uniqueFindings := deduplicateFindings(allFindings)

		output := &AgentReviewOutput{
			Findings:     uniqueFindings,
			AgentResults: agentResults,
			AggregatedResult: &AggregatedResult{
				TotalFindings:  len(allFindings),
				UniqueFindings: len(uniqueFindings),
				DuplicateCount: len(allFindings) - len(uniqueFindings),
				Findings:       uniqueFindings,
			},
		}

		return output, nil
	})
	chain.AppendLambda(aggregateNode, compose.WithNodeName("aggregate_findings"))

	return chain, nil
}

// MultiAgentRunner wraps the multi-agent workflow for use in the executor.
// The chain is compiled once during construction for efficiency.
type MultiAgentRunner struct {
	runnable     compose.Runnable[*AgentReviewInput, *AgentReviewOutput]
	agentManager *agent.AgentManager
}

// NewMultiAgentRunner creates a new multi-agent runner with pre-compiled chain
func NewMultiAgentRunner(agentManager *agent.AgentManager) (*MultiAgentRunner, error) {
	chain, err := BuildMultiAgentWorkflow(agentManager)
	if err != nil {
		return nil, fmt.Errorf("build workflow: %w", err)
	}

	// Compile once during construction for efficiency
	runnable, err := chain.Compile(context.Background())
	if err != nil {
		return nil, fmt.Errorf("compile workflow: %w", err)
	}

	return &MultiAgentRunner{
		runnable:     runnable,
		agentManager: agentManager,
	}, nil
}

// Execute runs the multi-agent review workflow
func (r *MultiAgentRunner) Execute(ctx context.Context, input *AgentReviewInput) (*AgentReviewOutput, error) {
	output, err := r.runnable.Invoke(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("execute workflow: %w", err)
	}

	return output, nil
}

// findingKey 用于 finding 去重的复合键（避免 fmt.Sprintf 分配）。
type findingKey struct {
	Path  string
	Line  int
	Title string
}

// deduplicateFindings removes duplicate findings based on path, line, and title
func deduplicateFindings(findings []domain.Finding) []domain.Finding {
	seen := make(map[findingKey]bool, len(findings))
	unique := make([]domain.Finding, 0, len(findings))

	for _, f := range findings {
		key := findingKey{Path: f.Path, Line: f.Line, Title: f.Title}
		if !seen[key] {
			seen[key] = true
			unique = append(unique, f)
		}
	}

	return unique
}

// MarshalAgentResultJSON marshals agent results to JSON
func MarshalAgentResultJSON(results []AgentResult) (json.RawMessage, error) {
	data, err := json.Marshal(results)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(data), nil
}
