package eino

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/yi-nology/code-review-service/internal/service/review/domain"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
	"github.com/cloudwego/eino/schema"
)

// --- Tool Input/Output Types ---

// OCRReviewInput is the input for OCR review tool
type OCRReviewInput struct {
	Dir        string `json:"dir" jsonschema:"description=Path to the workspace directory containing the code to review,required"`
	BaseSHA    string `json:"base_sha,omitempty" jsonschema:"description=Base commit SHA for diff comparison"`
	HeadSHA    string `json:"head_sha,omitempty" jsonschema:"description=Head commit SHA for diff comparison"`
	CommitSHA  string `json:"commit_sha,omitempty" jsonschema:"description=Specific commit SHA to review (mutually exclusive with base/head)"`
	Background string `json:"background,omitempty" jsonschema:"description=Additional context/background for the review"`
}

// OCRReviewOutput is the output for OCR review tool
type OCRReviewOutput struct {
	Result string `json:"result,omitempty"`
	Error  string `json:"error,omitempty"`
}

// ProviderInput is the input for provider tool
type ProviderInput struct {
	Provider        string `json:"provider" jsonschema:"description=Git platform provider (github, gitlab, gitea, gitcode, tencent),required"`
	Repo            string `json:"repo" jsonschema:"description=Repository identifier (owner/repo format),required"`
	PullRequestID   string `json:"pull_request_id,omitempty" jsonschema:"description=Pull request ID to review"`
	MergeRequestIID string `json:"merge_request_iid,omitempty" jsonschema:"description=Merge request IID (GitLab)"`
	BaseSHA         string `json:"base_sha,omitempty" jsonschema:"description=Base commit SHA"`
	HeadSHA         string `json:"head_sha,omitempty" jsonschema:"description=Head commit SHA"`
}

// ProviderOutput is the output for provider tool
type ProviderOutput struct {
	Dir      string `json:"dir,omitempty"`
	BaseSHA  string `json:"base_sha,omitempty"`
	HeadSHA  string `json:"head_sha,omitempty"`
	DiffSize int    `json:"diff_size,omitempty"`
	Error    string `json:"error,omitempty"`
}

// WritebackInput is the input for writeback tool
type WritebackInput struct {
	Provider        string `json:"provider" jsonschema:"description=Git platform provider,required"`
	Repo            string `json:"repo" jsonschema:"description=Repository identifier,required"`
	PullRequestID   string `json:"pull_request_id,omitempty" jsonschema:"description=Pull request ID"`
	MergeRequestIID string `json:"merge_request_iid,omitempty" jsonschema:"description=Merge request IID"`
	FindingsJSON    string `json:"findings_json" jsonschema:"description=JSON array of findings to publish,required"`
	Diff            string `json:"diff,omitempty" jsonschema:"description=The diff content for context"`
}

// WritebackOutput is the output for writeback tool
type WritebackOutput struct {
	Status    string `json:"status,omitempty"`
	Published int    `json:"published,omitempty"`
	Failed    int    `json:"failed,omitempty"`
	Error     string `json:"error,omitempty"`
}

// --- Tool Creation Functions ---

// NewOCRReviewTool creates an OCR review tool using agentkit pattern
func NewOCRReviewTool(runner domain.Runner) (tool.InvokableTool, error) {
	t, err := utils.InferTool("ocr_review", "Run OCR (Open Code Review) on a code workspace. Reviews code changes and returns findings with severity levels.",
		func(ctx context.Context, input *OCRReviewInput) (*OCRReviewOutput, error) {
			if input.Dir == "" {
				return &OCRReviewOutput{Error: "dir is required"}, nil
			}

			workspace := domain.Workspace{
				Dir:     input.Dir,
				BaseSHA: input.BaseSHA,
				HeadSHA: input.HeadSHA,
			}
			if input.Background != "" {
				workspace.OCR.Background = input.Background
			}

			result, err := runner.Run(ctx, workspace)
			if err != nil {
				return &OCRReviewOutput{Error: fmt.Sprintf("OCR review failed: %v", err)}, nil
			}

			resultJSON, err := json.Marshal(result)
			if err != nil {
				return &OCRReviewOutput{Error: fmt.Sprintf("marshal result: %v", err)}, nil
			}

			return &OCRReviewOutput{Result: string(resultJSON)}, nil
		})
	if err != nil {
		return nil, fmt.Errorf("create ocr_review tool: %w", err)
	}
	return t, nil
}

// NewProviderTool creates a provider tool using agentkit pattern
func NewProviderTool(provider domain.Provider) (tool.InvokableTool, error) {
	t, err := utils.InferTool("prepare_workspace", "Prepare a workspace by cloning a repository and resolving commit SHAs for code review.",
		func(ctx context.Context, input *ProviderInput) (*ProviderOutput, error) {
			req := domain.Request{
				Provider:        input.Provider,
				Repo:            input.Repo,
				PullRequestID:   input.PullRequestID,
				MergeRequestIID: input.MergeRequestIID,
				BaseSHA:         input.BaseSHA,
				HeadSHA:         input.HeadSHA,
			}

			workspace, err := provider.Prepare(ctx, req)
			if err != nil {
				return &ProviderOutput{Error: fmt.Sprintf("prepare workspace failed: %v", err)}, nil
			}

			return &ProviderOutput{
				Dir:      workspace.Dir,
				BaseSHA:  workspace.BaseSHA,
				HeadSHA:  workspace.HeadSHA,
				DiffSize: len(workspace.Diff),
			}, nil
		})
	if err != nil {
		return nil, fmt.Errorf("create prepare_workspace tool: %w", err)
	}
	return t, nil
}

// NewWritebackTool creates a writeback tool using agentkit pattern
func NewWritebackTool(publisher domain.WritebackPublisher) (tool.InvokableTool, error) {
	t, err := utils.InferTool("publish_findings", "Publish code review findings as inline comments on a pull/merge request.",
		func(ctx context.Context, input *WritebackInput) (*WritebackOutput, error) {
			req := domain.Request{
				Provider:        input.Provider,
				Repo:            input.Repo,
				PullRequestID:   input.PullRequestID,
				MergeRequestIID: input.MergeRequestIID,
			}

			var result domain.Result
			if err := json.Unmarshal([]byte(input.FindingsJSON), &result.Findings); err != nil {
				return &WritebackOutput{Error: fmt.Sprintf("parse findings: %v", err)}, nil
			}

			writebackResult, err := publisher.PublishWithDiff(ctx, req, result, input.Diff)
			if err != nil {
				return &WritebackOutput{Error: fmt.Sprintf("publish findings failed: %v", err)}, nil
			}

			return &WritebackOutput{
				Status:    "ok",
				Published: writebackResult.Published,
				Failed:    writebackResult.Failed,
			}, nil
		})
	if err != nil {
		return nil, fmt.Errorf("create publish_findings tool: %w", err)
	}
	return t, nil
}

// MustNewOCRReviewTool creates an OCR review tool and panics on error
func MustNewOCRReviewTool(runner domain.Runner) tool.InvokableTool {
	t, err := NewOCRReviewTool(runner)
	if err != nil {
		panic(err)
	}
	return t
}

// MustNewProviderTool creates a provider tool and panics on error
func MustNewProviderTool(provider domain.Provider) tool.InvokableTool {
	t, err := NewProviderTool(provider)
	if err != nil {
		panic(err)
	}
	return t
}

// MustNewWritebackTool creates a writeback tool and panics on error
func MustNewWritebackTool(publisher domain.WritebackPublisher) tool.InvokableTool {
	t, err := NewWritebackTool(publisher)
	if err != nil {
		panic(err)
	}
	return t
}

// --- Legacy Tool Info Functions (for backward compatibility) ---

// OCRReviewToolInfo returns the tool info for OCR review
func OCRReviewToolInfo() *schema.ToolInfo {
	return &schema.ToolInfo{
		Name: "ocr_review",
		Desc: "Run OCR (Open Code Review) on a code workspace. Reviews code changes and returns findings with severity levels.",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"dir": {
				Type:     "string",
				Desc:     "Path to the workspace directory containing the code to review",
				Required: true,
			},
			"base_sha": {
				Type: "string",
				Desc: "Base commit SHA for diff comparison",
			},
			"head_sha": {
				Type: "string",
				Desc: "Head commit SHA for diff comparison",
			},
			"commit_sha": {
				Type: "string",
				Desc: "Specific commit SHA to review (mutually exclusive with base/head)",
			},
			"background": {
				Type: "string",
				Desc: "Additional context/background for the review (requirements, conventions, etc.)",
			},
		}),
	}
}

// RunOCRReview executes the OCR review tool (legacy API)
func RunOCRReview(ctx context.Context, runner domain.Runner, arguments string) (string, error) {
	var args struct {
		Dir        string `json:"dir"`
		BaseSHA    string `json:"base_sha"`
		HeadSHA    string `json:"head_sha"`
		CommitSHA  string `json:"commit_sha"`
		Background string `json:"background"`
	}
	if err := json.Unmarshal([]byte(arguments), &args); err != nil {
		return "", fmt.Errorf("parse arguments: %w", err)
	}

	if args.Dir == "" {
		return "", fmt.Errorf("dir is required")
	}

	workspace := domain.Workspace{
		Dir:     args.Dir,
		BaseSHA: args.BaseSHA,
		HeadSHA: args.HeadSHA,
	}
	if args.Background != "" {
		workspace.OCR.Background = args.Background
	}

	result, err := runner.Run(ctx, workspace)
	if err != nil {
		return "", fmt.Errorf("OCR review failed: %w", err)
	}

	resultJSON, err := json.Marshal(result)
	if err != nil {
		return "", fmt.Errorf("marshal result: %w", err)
	}

	return string(resultJSON), nil
}
