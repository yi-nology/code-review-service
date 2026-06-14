package agent

import (
	"context"

	"github.com/yi-nology/code-review-service/internal/service/review"
	"github.com/yi-nology/code-review-service/internal/service/runner"
)

// OCRRunnerFactory creates OCR runners with different configurations
type OCRRunnerFactory struct {
	baseConfig runner.OpenCodeReviewConfig
}

// NewOCRRunnerFactory creates a new OCR runner factory
func NewOCRRunnerFactory(baseConfig runner.OpenCodeReviewConfig) *OCRRunnerFactory {
	return &OCRRunnerFactory{baseConfig: baseConfig}
}

// CreateRunner creates a runner with the given background
func (f *OCRRunnerFactory) CreateRunner(background string) ReviewRunner {
	config := f.baseConfig
	config.Background = background
	return &OCRRunnerAdapter{runner: runner.NewOpenCodeReviewRunner(config)}
}

// OCRRunnerAdapter adapts runner.OpenCodeReviewRunner to agent.ReviewRunner
type OCRRunnerAdapter struct {
	runner *runner.OpenCodeReviewRunner
}

// Run runs the OCR review
func (a *OCRRunnerAdapter) Run(ctx context.Context, workspace ReviewWorkspace) (ReviewResult, error) {
	// Convert agent workspace to review workspace
	reviewWorkspace := review.Workspace{
		Dir:     workspace.Dir,
		BaseSHA: workspace.BaseSHA,
		HeadSHA: workspace.HeadSHA,
		Diff:    workspace.Diff,
	}

	// Run OCR
	result, err := a.runner.Run(ctx, reviewWorkspace)
	if err != nil {
		return ReviewResult{}, err
	}

	// Convert review result to agent result
	findings := make([]ReviewFinding, len(result.Findings))
	for i, f := range result.Findings {
		findings[i] = ReviewFinding{
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

	return ReviewResult{
		Summary:  result.Summary,
		Findings: findings,
	}, nil
}
