package adapter

import (
	"context"

	"github.com/yi-nology/code-review-service/internal/service/agent"
	"github.com/yi-nology/code-review-service/internal/service/review"
)

// ReviewRunnerAdapter adapts review.Runner to agent.ReviewRunner
type ReviewRunnerAdapter struct {
	runner review.Runner
}

// NewReviewRunnerAdapter creates a new adapter
func NewReviewRunnerAdapter(runner review.Runner) *ReviewRunnerAdapter {
	return &ReviewRunnerAdapter{runner: runner}
}

// Run runs the review
func (a *ReviewRunnerAdapter) Run(ctx context.Context, workspace agent.ReviewWorkspace) (agent.ReviewResult, error) {
	// Convert workspace
	reviewWorkspace := review.Workspace{
		Dir:     workspace.Dir,
		BaseSHA: workspace.BaseSHA,
		HeadSHA: workspace.HeadSHA,
		Diff:    workspace.Diff,
	}

	// Run review
	result, err := a.runner.Run(ctx, reviewWorkspace)
	if err != nil {
		return agent.ReviewResult{}, err
	}

	// Convert findings
	findings := make([]agent.ReviewFinding, len(result.Findings))
	for i, f := range result.Findings {
		findings[i] = agent.ReviewFinding{
			Severity:       string(f.Severity),
			Path:           f.Path,
			Line:           f.Line,
			StartLine:      f.StartLine,
			EndLine:        f.EndLine,
			Title:          f.Title,
			Body:           f.Body,
			ExistingCode:   f.ExistingCode,
			SuggestionCode: f.SuggestionCode,
		}
	}

	return agent.ReviewResult{
		Summary:  result.Summary,
		Findings: findings,
	}, nil
}
