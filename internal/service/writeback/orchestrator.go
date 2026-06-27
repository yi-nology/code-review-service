package writeback

import (
	"context"
	"fmt"
	"strings"

	"github.com/yi-nology/code-review-service/internal/repository"
	"github.com/yi-nology/code-review-service/internal/repository/postgres/models"
	"github.com/yi-nology/code-review-service/internal/service/review"
	"github.com/yi-nology/code-review-service/internal/service/types"

	"github.com/oekit/logx"
)

type OrchestratorConfig struct {
	Adapters    map[string]Adapter
	ItemRepo    repository.WritebackItemRepository
	ChangeRepo  repository.ChangeRevisionRepository
	MaxComments int
	SummaryOnly bool
}

type Orchestrator struct {
	adapters    map[string]Adapter
	itemRepo    repository.WritebackItemRepository
	changeRepo  repository.ChangeRevisionRepository
	maxComments int
	summaryOnly bool
}

func NewOrchestrator(cfg OrchestratorConfig) *Orchestrator {
	if cfg.MaxComments <= 0 {
		cfg.MaxComments = 20
	}
	return &Orchestrator{
		adapters:    cfg.Adapters,
		itemRepo:    cfg.ItemRepo,
		changeRepo:  cfg.ChangeRepo,
		maxComments: cfg.MaxComments,
		summaryOnly: cfg.SummaryOnly,
	}
}

func (o *Orchestrator) Publish(ctx context.Context, req review.Request, result review.Result, diff string) (*review.WritebackResult, error) {
	adapter, ok := o.adapters[req.Provider]
	if !ok {
		return nil, fmt.Errorf("no writeback adapter for provider %q", req.Provider)
	}

	number := types.ResolvePRNumber(req.Provider, req.PullRequestID, req.MergeRequestIID)
	if number == 0 {
		return nil, fmt.Errorf("cannot determine PR/MR number for writeback")
	}

	cr := ChangeRequest{
		Provider:   req.Provider,
		Repository: strings.TrimPrefix(req.Repo, "/"),
		Number:     number,
		BaseSHA:    req.BaseSHA,
		HeadSHA:    req.HeadSHA,
	}

	caps, err := adapter.Capabilities(ctx, cr)
	if err != nil {
		return nil, fmt.Errorf("check capabilities: %w", err)
	}

	summaryBody := FormatSummaryBody(result.Summary, result.Findings, o.maxComments)

	var diffIndex *DiffIndex
	if !o.summaryOnly && diff != "" {
		logx.Info("parsing diff for writeback validation", "diff_size", len(diff))
		diffIndex, err = ParseUnifiedDiff(diff)
		if err != nil {
			logx.Warn("failed to parse diff for writeback validation", "error", err)
		}
	} else {
		logx.Info("skipping diff parsing", "summaryOnly", o.summaryOnly, "diff_empty", diff == "")
	}

	var comments []Comment
	if !o.summaryOnly && len(result.Findings) > 0 {
		validated := ValidateFindings(result.Findings, diffIndex)
		limit := len(validated)
		if o.maxComments > 0 && limit > o.maxComments {
			limit = o.maxComments
		}

		for i := 0; i < limit; i++ {
			vc := validated[i]
			if !vc.ValidLocation {
				continue
			}
			if !caps.MultiLineComments && vc.Range.End > vc.Range.Start+1 {
				continue
			}

			body := FormatInlineComment(
				findSeverity(result.Findings, i),
				findTitle(result.Findings, i),
				vc.Body,
				vc.Suggestion,
				findExistingCode(result.Findings, i),
				vc.Range.Start,
				vc.Key,
			)
			comments = append(comments, Comment{
				Key:        vc.Key,
				Path:       vc.Path,
				Body:       body,
				Range:      vc.Range,
				Suggestion: vc.Suggestion,
			})
		}
	}

	revEvent := EventComment
	if !caps.BatchReview {
		revEvent = EventReview
	}

	rev := Review{
		ChangeRequest: cr,
		Summary:       summaryBody,
		Event:         revEvent,
		Comments:      comments,
	}

	wbResult, err := adapter.Publish(ctx, rev)
	if err != nil {
		return nil, fmt.Errorf("publish writeback: %w", err)
	}

	if wbResult.ReviewID == "" && len(wbResult.Comments) == 0 {
		logx.Info("writeback skipped (stale or empty)")
		return &review.WritebackResult{Skipped: true}, nil
	}

	serviceReviewID := strings.TrimSpace(req.ReviewID)
	if serviceReviewID == "" {
		serviceReviewID = wbResult.ReviewID
	}

	published, failed := 0, 0
	for _, cr := range wbResult.Comments {
		status := "published"
		if !cr.Inline {
			status = "downgraded"
		}
		if cr.Error != "" {
			status = "failed"
			failed++
		} else {
			published++
		}
		_ = o.itemRepo.Create(ctx, &models.WritebackItem{
			ReviewID:           serviceReviewID,
			FindingKey:         cr.Key,
			Kind:               "inline",
			ProviderExternalID: cr.ExternalID,
			ProviderURL:        cr.URL,
			Status:             status,
			Attempts:           1,
			LastError:          cr.Error,
		})
	}

	logx.Info("writeback completed",
		"provider", req.Provider,
		"repo", req.Repo,
		"number", number,
		"comments", len(comments),
		"published", published,
		"failed", failed,
	)
	return &review.WritebackResult{Published: published, Failed: failed}, nil
}

func findSeverity(findings []review.Finding, i int) review.Severity {
	if i < len(findings) {
		return findings[i].Severity
	}
	return review.SeverityInfo
}

func findTitle(findings []review.Finding, i int) string {
	if i < len(findings) {
		return findings[i].Title
	}
	return ""
}

func findExistingCode(findings []review.Finding, i int) string {
	if i < len(findings) {
		return findings[i].ExistingCode
	}
	return ""
}
