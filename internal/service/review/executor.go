package review

import (
	"context"
	"encoding/json"
	"os"
	"strings"

	"github.com/yi-nology/code-review-service/internal/repository/postgres/models"
	"github.com/yi-nology/code-review-service/internal/service/policy"

	"github.com/cloudwego/hertz/pkg/common/hlog"
	"github.com/oekit/pkg/slicex"
)

func (s *Service) run(ctx context.Context, id string, req Request, provider Provider) (Response, error) {
	if err := s.updateStatus(ctx, id, "running"); err != nil {
		return Response{}, err
	}

	workspace, err := provider.Prepare(ctx, req)
	if err != nil {
		s.recordFailure(ctx, id, req, err)
		return Response{}, err
	}
	s.appendHistory(ctx, id, "repository", "completed", "repository and change scope prepared", map[string]any{
		"base_sha": workspace.BaseSHA,
		"head_sha": workspace.HeadSHA,
	})

	// Write resolved SHAs back to the review record so the API returns them
	if workspace.BaseSHA != "" || workspace.HeadSHA != "" {
		req.BaseSHA = workspace.BaseSHA
		req.HeadSHA = workspace.HeadSHA
		s.updateRequestSHAs(ctx, id, req)
	}

	projectPolicy := models.ProjectPolicy{FailOn: "critical"}
	if s.projectRepo != nil {
		policy, err := s.projectRepo.GetByProviderRepo(ctx, req.Provider, req.Repo)
		if err == nil && policy != nil {
			projectPolicy = *policy
		}
	}

	var ignorePaths []string
	if len(projectPolicy.IgnorePaths) > 0 {
		if err := json.Unmarshal(projectPolicy.IgnorePaths, &ignorePaths); err != nil {
			hlog.Warn("failed to decode ignore_paths", "error", err)
		}
	}
	if len(ignorePaths) > 0 {
		workspace, err = applyProjectScope(ctx, workspace, ignorePaths)
		if err != nil {
			s.recordFailure(ctx, id, req, err)
			return Response{}, err
		}
		s.appendHistory(ctx, id, "scope", "completed", "project review policy applied", map[string]any{
			"changed_files":  workspace.Scope.ChangedFiles,
			"reviewed_files": workspace.Scope.ReviewedFiles,
			"excluded_files": workspace.Scope.ExcludedFiles,
		})
	}

	if s.globalRulesRepo != nil {
		rulePath, cleanup, ruleErr := s.resolveRuleFile(ctx, projectPolicy)
		if ruleErr != nil {
			hlog.Warn("failed to resolve rule file", "error", ruleErr)
		} else if rulePath != "" {
			workspace.RulePath = rulePath
			if cleanup != nil {
				defer cleanup()
			}
		}
	}

	if workspace.Dir != "" {
		defer func() {
			if err := os.RemoveAll(workspace.Dir); err != nil {
				hlog.Warn("failed to remove review workspace", "dir", workspace.Dir, "error", err)
			}
		}()
	}

	s.appendHistory(ctx, id, "ocr", "running", "code review model invoked", nil)
	result, err := s.runner.Run(ctx, workspace)
	if err != nil {
		s.recordFailure(ctx, id, req, err)
		return Response{}, err
	}
	s.appendHistory(ctx, id, "ocr", "completed", "code review model analysis completed", map[string]any{
		"files_reviewed": result.OCR.FilesReviewed,
		"comments":       result.OCR.Comments,
		"total_tokens":   result.OCR.TotalTokens,
		"elapsed":        result.OCR.Elapsed,
	})
	result.Scope = workspace.Scope
	result.Scope.ChangedLines = CountDiffLines(workspace.Diff)

	report := BuildReport(&result)
	analysis := BuildAnalysis(result)
	gateEngine := s.policy
	if projectPolicy.FailOn != "" {
		gateEngine = policy.NewEngine(policy.Config{FailOn: projectPolicy.FailOn})
	}
	gate := gateEngine.Evaluate(toPolicyFindings(result.Findings))
	resp := Response{
		ReviewID: id,
		Status:   "success",
		Summary:  result.Summary,
		Report:   report,
		Analysis: analysis,
		Findings: result.Findings,
		Gate:     CompletedGate(gate.Passed, gate.Reason),
	}

	callbackMode := req.CallbackMode
	hasPR := strings.TrimSpace(req.PullRequestID) != "" || strings.TrimSpace(req.MergeRequestIID) != ""
	if !hasPR && (callbackMode == "" || callbackMode == "comment") {
		callbackMode = "none"
	}

	if callbackMode != "none" && callbackMode != "summary" {
		if s.writeback != nil {
			writebackReq := req
			if workspace.OriginalHeadSHA != "" {
				writebackReq.HeadSHA = workspace.OriginalHeadSHA
			}
			if err := s.writeback.PublishWithDiff(ctx, writebackReq, result, workspace.Diff); err != nil {
				hlog.Warn("writeback publish failed, falling back to legacy publisher", "error", err)
				if err := s.publisher.Publish(ctx, req, result); err != nil {
					s.recordFailure(ctx, id, req, err)
					return Response{}, err
				}
			}
		} else if err := s.publisher.Publish(ctx, req, result); err != nil {
			s.recordFailure(ctx, id, req, err)
			return Response{}, err
		}
	}

	if err := s.updateWithRawOCR(ctx, id, req, resp, result.RawOCR); err != nil {
		return Response{}, err
	}

	return resp, nil
}

func (s *Service) recordFailure(ctx context.Context, id string, req Request, runErr error) {
	response := Response{
		ReviewID: id,
		Status:   "failed",
		Summary:  "review failed",
		Gate:     ErrorGate("review execution failed"),
		Error:    runErr.Error(),
	}
	if err := s.updateWithRawOCR(ctx, id, req, response, nil); err != nil {
		hlog.Error("failed to persist review failure", "review_id", id, "error", err)
	}
}

func toPolicyFindings(findings []Finding) []policy.Finding {
	return slicex.Map(findings, func(f Finding) policy.Finding {
		return policy.Finding{Severity: string(f.Severity), Path: f.Path}
	})
}

