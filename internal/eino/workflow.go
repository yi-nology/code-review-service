package eino

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/yi-nology/code-review-service/internal/domain/projectpolicy"
	"github.com/yi-nology/code-review-service/internal/service/policy"
	"github.com/yi-nology/code-review-service/internal/service/review"
	"github.com/yi-nology/code-review-service/internal/service/review/domain"
	"github.com/yi-nology/code-review-service/internal/service/types"

	"github.com/cloudwego/eino/compose"
)

// ReviewInput is the input to the review workflow
type ReviewInput struct {
	ReviewID      string                      `json:"review_id"`
	Request       domain.Request              `json:"request"`
	Workspace     domain.Workspace            `json:"workspace"`
	ProjectPolicy projectpolicy.ProjectPolicy `json:"project_policy"`
	GatePolicy    *policy.Engine              `json:"-"`
}

// ReviewOutput is the output from the review workflow
type ReviewOutput struct {
	Response domain.Response `json:"response"`
	Result   domain.Result   `json:"result"`
}

// ReviewContext carries shared state through the workflow nodes
type ReviewContext struct {
	Input     ReviewInput
	Workspace domain.Workspace
	Result    *domain.Result
	Report    *domain.Report
	Analysis  *domain.Analysis
	Gate      *domain.Gate
}

// BuildReviewWorkflow creates an eino compose.Chain that orchestrates the review execution.
// This replaces the hand-written prepare → OCR → save → publish call chain.
//
// The workflow has 4 nodes:
// 1. prepareWorkspace - Validate workspace and apply project policy
// 2. runOCR - Execute OCR code review
// 3. buildResult - Compute report, analysis, gate verdict
// 4. assembleOutput - Assemble final output
func BuildReviewWorkflow(runner domain.Runner, policyEngine *policy.Engine) (*compose.Chain[*ReviewInput, *ReviewOutput], error) {
	chain := compose.NewChain[*ReviewInput, *ReviewOutput]()

	// Node 1: Prepare workspace (already done by provider, this validates and enriches)
	prepareNode := compose.InvokableLambda(func(ctx context.Context, input *ReviewInput) (*ReviewContext, error) {
		if input.Workspace.Dir == "" {
			return nil, fmt.Errorf("workspace directory is required")
		}

		// Apply project policy OCR overrides
		input.Workspace.OCR = review.ProjectPolicyToOCROverrides(input.ProjectPolicy)

		return &ReviewContext{
			Input:     *input,
			Workspace: input.Workspace,
		}, nil
	})
	chain.AppendLambda(prepareNode, compose.WithNodeName("prepare_workspace"))

	// Node 2: Run OCR code review
	ocrNode := compose.InvokableLambda(func(ctx context.Context, rc *ReviewContext) (*ReviewContext, error) {
		if runner == nil {
			return nil, fmt.Errorf("OCR runner not configured")
		}

		result, err := runner.Run(ctx, rc.Workspace)
		if err != nil {
			return nil, fmt.Errorf("OCR execution failed: %w", err)
		}

		rc.Result = &result
		return rc, nil
	})
	chain.AppendLambda(ocrNode, compose.WithNodeName("run_ocr"))

	// Node 3: Build report, analysis, and gate verdict
	buildNode := compose.InvokableLambda(func(ctx context.Context, rc *ReviewContext) (*ReviewContext, error) {
		if rc.Result == nil {
			return nil, fmt.Errorf("OCR result is nil")
		}

		// Compute stable fingerprints
		types.ComputeFingerprints(rc.Result.Findings)

		// Build report
		report := review.BuildReport(rc.Result)
		rc.Report = &report

		// Build analysis
		analysis := review.BuildAnalysis(*rc.Result)
		rc.Analysis = &analysis

		// Evaluate gate policy
		gateEngine := policyEngine
		if gateEngine == nil {
			gateEngine = policy.NewEngine(policy.Config{})
		}
		if rc.Input.ProjectPolicy.FailOn != "" {
			gateEngine = policy.NewEngine(policy.Config{FailOn: rc.Input.ProjectPolicy.FailOn})
		}

		policyFindings := toPolicyFindings(rc.Result.Findings)
		gateResult := gateEngine.Evaluate(policyFindings)

		passed := gateResult.Passed
		rc.Gate = &domain.Gate{
			Passed: &passed,
			Reason: gateResult.Reason,
		}

		return rc, nil
	})
	chain.AppendLambda(buildNode, compose.WithNodeName("build_result"))

	// Node 4: Assemble final output
	outputNode := compose.InvokableLambda(func(ctx context.Context, rc *ReviewContext) (*ReviewOutput, error) {
		// Nil checks for safety
		if rc.Result == nil {
			return nil, fmt.Errorf("OCR result is nil")
		}
		if rc.Report == nil {
			return nil, fmt.Errorf("report is nil")
		}
		if rc.Analysis == nil {
			return nil, fmt.Errorf("analysis is nil")
		}
		if rc.Gate == nil {
			return nil, fmt.Errorf("gate is nil")
		}

		now := time.Now().UTC()

		resp := domain.Response{
			ReviewID: rc.Input.ReviewID,
			Status:   "success",
			Summary:  rc.Result.Summary,
			Gate: domain.Gate{
				Passed: rc.Gate.Passed,
				Reason: rc.Gate.Reason,
			},
			Report:   *rc.Report,
			Analysis: *rc.Analysis,
			Findings: rc.Result.Findings,
			History: []domain.Event{
				{
					Stage:     "repository",
					Status:    "completed",
					Message:   "workspace prepared",
					CreatedAt: now,
				},
				{
					Stage:   "ocr_review",
					Status:  "completed",
					Message: "code review completed",
					Details: map[string]any{
						"files_reviewed": rc.Result.OCR.FilesReviewed,
						"comments":       rc.Result.OCR.Comments,
						"total_tokens":   rc.Result.OCR.TotalTokens,
						"elapsed":        rc.Result.OCR.Elapsed,
					},
					CreatedAt: now,
				},
			},
		}

		return &ReviewOutput{
			Response: resp,
			Result:   *rc.Result,
		}, nil
	})
	chain.AppendLambda(outputNode, compose.WithNodeName("assemble_output"))

	return chain, nil
}

// ReviewWorkflowRunner wraps the eino workflow as a reusable runner.
// The chain is compiled once during construction for efficiency.
type ReviewWorkflowRunner struct {
	runnable compose.Runnable[*ReviewInput, *ReviewOutput]
}

// NewReviewWorkflowRunner creates a new workflow runner with pre-compiled chain
func NewReviewWorkflowRunner(runner domain.Runner, policyEngine *policy.Engine) (*ReviewWorkflowRunner, error) {
	chain, err := BuildReviewWorkflow(runner, policyEngine)
	if err != nil {
		return nil, fmt.Errorf("build workflow: %w", err)
	}

	// Compile once during construction for efficiency
	runnable, err := chain.Compile(context.Background())
	if err != nil {
		return nil, fmt.Errorf("compile workflow: %w", err)
	}

	return &ReviewWorkflowRunner{
		runnable: runnable,
	}, nil
}

// Execute runs the review workflow and returns the response
func (r *ReviewWorkflowRunner) Execute(ctx context.Context, input *ReviewInput) (domain.Response, error) {
	output, err := r.runnable.Invoke(ctx, input)
	if err != nil {
		return domain.Response{}, fmt.Errorf("execute workflow: %w", err)
	}

	return output.Response, nil
}

// ExecuteFull runs the review workflow and returns the full output
func (r *ReviewWorkflowRunner) ExecuteFull(ctx context.Context, input *ReviewInput) (*ReviewOutput, error) {
	output, err := r.runnable.Invoke(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("execute workflow: %w", err)
	}

	return output, nil
}

// MarshalResultJSON marshals the result to JSON for persistence
func MarshalResultJSON(result domain.Result) (json.RawMessage, error) {
	data, err := json.Marshal(result)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(data), nil
}
