package review

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/oekit/concurrency/async"

	"github.com/yi-nology/code-review-service/internal/repository/postgres/models"
	"github.com/yi-nology/code-review-service/internal/repository/repoerr"
	"github.com/yi-nology/code-review-service/internal/service/review/domain"
)

type ModelCompareCandidate struct {
	Label        string `json:"label"`
	Provider     string `json:"provider,omitempty"`
	URL          string `json:"url,omitempty"`
	Model        string `json:"model"`
	AuthToken    string `json:"auth_token,omitempty"`
	AuthHeader   string `json:"auth_header,omitempty"`
	ExtraHeaders string `json:"extra_headers,omitempty"`
	ExtraBody    string `json:"extra_body,omitempty"`
}

type ModelCompareOptions struct {
	Candidates []ModelCompareCandidate `json:"candidates"`
}

type ModelCompareResult struct {
	ReviewID   string                     `json:"review_id"`
	Candidates []ModelCompareCandidateRun `json:"candidates"`
}

type ModelCompareCandidateRun struct {
	Label        string                `json:"label"`
	Provider     string                `json:"provider,omitempty"`
	URL          string                `json:"url,omitempty"`
	Model        string                `json:"model"`
	Success      bool                  `json:"success"`
	Score        int                   `json:"score,omitempty"`
	Grade        string                `json:"grade,omitempty"`
	Findings     int                   `json:"findings"`
	Warnings     int                   `json:"warnings"`
	TotalTokens  int64                 `json:"total_tokens,omitempty"`
	InputTokens  int64                 `json:"input_tokens,omitempty"`
	OutputTokens int64                 `json:"output_tokens,omitempty"`
	Elapsed      string                `json:"elapsed,omitempty"`
	DurationMs   int64                 `json:"duration_ms"`
	Summary      string                `json:"summary,omitempty"`
	Error        string                `json:"error,omitempty"`
	Counts       domain.SeverityCounts `json:"counts,omitempty"`
}

func (s *Service) CompareModels(ctx context.Context, reviewID string, opts ModelCompareOptions) (*ModelCompareResult, bool, error) {
	record, found, err := s.Get(ctx, reviewID)
	if err != nil || !found {
		return nil, found, err
	}
	if len(opts.Candidates) == 0 {
		return nil, true, fmt.Errorf("candidates is required")
	}
	if len(opts.Candidates) > 4 {
		return nil, true, fmt.Errorf("candidates exceeds maximum (4)")
	}
	if s.runner == nil {
		return nil, true, fmt.Errorf("runner feature not configured")
	}
	if s.Providers == nil {
		return nil, true, fmt.Errorf("providers not configured")
	}
	provider, ok := s.Providers.Get(record.Request.Provider)
	if !ok {
		return nil, true, fmt.Errorf("%w: %s", domain.ErrUnsupportedProvider, record.Request.Provider)
	}

	projectPolicy := models.ProjectPolicy{FailOn: "critical"}
	if s.projectRepo != nil {
		policyRecord, err := s.projectRepo.GetByProviderRepo(ctx, record.Request.Provider, record.Request.Repo)
		if err != nil && !errors.Is(err, repoerr.ErrNotFound) {
			return nil, true, err
		}
		if policyRecord != nil {
			projectPolicy = *policyRecord
		}
	}

	// 各 candidate 的 OCR 运行彼此独立（各自 clone 工作区 + LLM 调用），
	// 并行执行以避免 N 个候选模型延迟线性叠加。结果按索引回填保持顺序稳定。
	runs := make([]ModelCompareCandidateRun, len(opts.Candidates))
	var wg sync.WaitGroup
	for i, candidate := range opts.Candidates {
		i, candidate := i, candidate
		wg.Add(1)
		async.GoSafe(func() {
			defer wg.Done()
			runs[i] = s.runModelCandidate(ctx, provider, record.Request, projectPolicy, candidate)
		})
	}
	wg.Wait()
	return &ModelCompareResult{ReviewID: record.ID, Candidates: runs}, true, nil
}

func (s *Service) runModelCandidate(ctx context.Context, provider domain.Provider, req domain.Request, projectPolicy models.ProjectPolicy, candidate ModelCompareCandidate) ModelCompareCandidateRun {
	started := time.Now()
	run := ModelCompareCandidateRun{
		Label:    firstNonBlankLocal(candidate.Label, candidate.Model),
		Provider: candidate.Provider,
		URL:      candidate.URL,
		Model:    candidate.Model,
	}
	if strings.TrimSpace(candidate.Model) == "" {
		run.Error = "model is required"
		run.DurationMs = time.Since(started).Milliseconds()
		return run
	}
	workspace, err := provider.Prepare(ctx, req)
	if err != nil {
		run.Error = err.Error()
		run.DurationMs = time.Since(started).Milliseconds()
		return run
	}
	if workspace.Dir != "" {
		defer func() { _ = os.RemoveAll(workspace.Dir) }()
	}
	workspace.OCR = ProjectPolicyToOCROverrides(projectPolicy)
	workspace.OCR.Provider = candidate.Provider
	workspace.OCR.URL = candidate.URL
	workspace.OCR.Model = candidate.Model
	workspace.OCR.AuthToken = candidate.AuthToken
	workspace.OCR.AuthHeader = candidate.AuthHeader
	workspace.OCR.ExtraHeaders = candidate.ExtraHeaders
	workspace.OCR.ExtraBody = candidate.ExtraBody

	if s.globalRulesRepo != nil {
		rulePath, cleanup, ruleErr := ResolveRuleFile(ctx, projectPolicy, s.rulePackageRepo, s.globalRulesRepo)
		if ruleErr == nil && rulePath != "" {
			workspace.RulePath = rulePath
			if cleanup != nil {
				defer cleanup()
			}
		}
	}

	ocrResult, err := s.runner.Run(ctx, workspace)
	run.DurationMs = time.Since(started).Milliseconds()
	if err != nil {
		run.Error = err.Error()
		return run
	}
	report := BuildReport(&ocrResult)
	run.Success = true
	run.Score = report.Score.Value
	run.Grade = report.Score.Grade
	run.Findings = len(ocrResult.Findings)
	run.Warnings = len(ocrResult.Warnings)
	run.TotalTokens = ocrResult.OCR.TotalTokens
	run.InputTokens = ocrResult.OCR.InputTokens
	run.OutputTokens = ocrResult.OCR.OutputTokens
	run.Elapsed = ocrResult.OCR.Elapsed
	run.Summary = ocrResult.Summary
	run.Counts = report.Counts
	return run
}

func firstNonBlankLocal(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
