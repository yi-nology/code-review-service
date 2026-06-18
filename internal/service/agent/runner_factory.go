package agent

import (
	"context"

	"github.com/yi-nology/code-review-service/internal/service/review"
	"github.com/yi-nology/code-review-service/internal/service/runner"
)

type OCRRunnerFactory struct {
	baseConfig runner.OpenCodeReviewConfig
}

func NewOCRRunnerFactory(baseConfig runner.OpenCodeReviewConfig) *OCRRunnerFactory {
	return &OCRRunnerFactory{baseConfig: baseConfig}
}

func (f *OCRRunnerFactory) CreateRunner(background string) reviewRunner {
	config := f.baseConfig
	config.Background = background
	return &ocrRunnerAdapter{runner: runner.NewOpenCodeReviewRunner(config)}
}

type ocrRunnerAdapter struct {
	runner *runner.OpenCodeReviewRunner
}

func (a *ocrRunnerAdapter) Run(ctx context.Context, workspace reviewWorkspace) (reviewResult, error) {
	reviewWs := review.Workspace{
		Dir:     workspace.Dir,
		BaseSHA: workspace.BaseSHA,
		HeadSHA: workspace.HeadSHA,
		Diff:    workspace.Diff,
	}

	result, err := a.runner.Run(ctx, reviewWs)
	if err != nil {
		return reviewResult{}, err
	}

	findings := make([]reviewFinding, len(result.Findings))
	for i, f := range result.Findings {
		findings[i] = reviewFinding{
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

	return reviewResult{
		Summary:  result.Summary,
		Findings: findings,
	}, nil
}
