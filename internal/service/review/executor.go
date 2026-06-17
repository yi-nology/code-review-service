package review

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/yi-nology/code-review-service/internal/repository/postgres/models"
	"github.com/yi-nology/code-review-service/internal/service/policy"
	"github.com/yi-nology/code-review-service/internal/service/progress"
	"github.com/yi-nology/code-review-service/internal/service/reviewmap"

	"github.com/oekit/pkg/slicex"
	"github.com/cloudwego/hertz/pkg/common/hlog"
)

func (s *Service) run(ctx context.Context, id string, req Request, provider Provider, progressReporter progress.ProgressReporter) (Response, error) {
	if err := s.updateStatus(ctx, id, "running"); err != nil {
		return Response{}, err
	}

	// Inject progress reporter into context for the runner
	if progressReporter != nil {
		ctx = progress.WithProgressReporter(ctx, progressReporter)
	}

	// Save initial OCR input (will be updated with workspace info later)
	initialInput := OCRInput{
		Binary: "ocr",
		Workspace: WorkspaceInfo{
			Provider: req.Provider,
			Repo:     req.Repo,
			BaseSHA:  req.BaseSHA,
			HeadSHA:  req.HeadSHA,
		},
		StartedAt: time.Now().UTC(),
	}
	if err := s.updateWithOCRInput(ctx, id, initialInput); err != nil {
		hlog.Warn("failed to save initial OCR input", "review_id", id, "error", err)
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
	workspace.OCR = projectPolicyOCROverrides(projectPolicy)

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

	// Update OCR input with full workspace info
	ocrInput := buildOCRInput(workspace, req)
	if err := s.updateWithOCRInput(ctx, id, ocrInput); err != nil {
		hlog.Warn("failed to update OCR input", "review_id", id, "error", err)
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

	// Generate Review Map if repo is available
	if s.reviewMapRepo != nil && workspace.Diff != "" {
		s.generateAndSaveReviewMap(ctx, id, req, workspace, result)
	}

	return resp, nil
}

func projectPolicyOCROverrides(policy models.ProjectPolicy) OCROverrides {
	return OCROverrides{
		Enabled:         policy.OCREnabled,
		Concurrency:     policy.OCRConcurrency,
		TimeoutMinutes:  policy.OCRTimeoutMinutes,
		MaxTools:        policy.OCRMaxTools,
		DefaultSeverity: policy.OCRDefaultSeverity,
		Background:      policy.OCRBackground,
	}
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

// generateAndSaveReviewMap generates a review map and saves it to the database
func (s *Service) generateAndSaveReviewMap(ctx context.Context, id string, req Request, workspace Workspace, result Result) {
	// Convert findings to reviewmap format
	var findings []reviewmap.Finding
	for i, f := range result.Findings {
		findings = append(findings, reviewmap.Finding{
			ID:         fmt.Sprintf("finding-%d", i),
			Severity:   string(f.Severity),
			Category:   f.Category,
			Path:       f.Path,
			Line:       f.Line,
			EndLine:    f.EndLine,
			Title:      f.Title,
			Body:       f.Body,
			Suggestion: f.SuggestionCode,
			Confidence: 0.8,
			Agents:     []string{"default"},
		})
	}

	// Generate review map
	generator := reviewmap.NewGenerator()
	reviewMap, err := generator.Generate(id, workspace.Diff, findings)
	if err != nil {
		hlog.Warn("failed to generate review map", "review_id", id, "error", err)
		return
	}

	// Save to database
	dbMap := &models.ReviewMap{
		MapID:          reviewMap.ID,
		ReviewID:       id,
		SessionID:      id, // Use review ID as session ID for single-agent mode
		Title:          reviewMap.Title,
		Description:    reviewMap.Description,
		Sections:       mustMarshal(reviewMap.Sections),
		Dependencies:   mustMarshal(reviewMap.Dependencies),
		Summary:        reviewMap.Summary,
		FileCount:      reviewMap.FileCount,
		SectionCount:   reviewMap.SectionCount,
		MermaidDiagram: reviewMap.ToMermaid(),
	}

	if err := s.reviewMapRepo.Create(ctx, dbMap); err != nil {
		hlog.Warn("failed to save review map", "review_id", id, "error", err)
		return
	}

	hlog.Info("review map generated and saved", "review_id", id, "map_id", reviewMap.ID, "sections", reviewMap.SectionCount, "files", reviewMap.FileCount)
}

func buildOCRInput(workspace Workspace, req Request) OCRInput {
	return OCRInput{
		Binary: "ocr",
		Args: []string{
			"review",
			"--repo", workspace.Dir,
			"--from", workspace.BaseSHA,
			"--to", workspace.HeadSHA,
			"--format", "json",
		},
		Workspace: WorkspaceInfo{
			Provider: req.Provider,
			Repo:     req.Repo,
			BaseSHA:  workspace.BaseSHA,
			HeadSHA:  workspace.HeadSHA,
			Dir:      workspace.Dir,
		},
		RulePath:  workspace.RulePath,
		StartedAt: time.Now().UTC(),
	}
}
