package writeback

import (
	"context"
	"fmt"
	"strings"

	wbitem "github.com/yi-nology/code-review-service/internal/domain/writeback"
	"github.com/yi-nology/code-review-service/internal/repository"
	"github.com/yi-nology/code-review-service/internal/service/review/domain"
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

type PublishOptions struct {
	Force bool
}

type Orchestrator struct {
	adapters    map[string]Adapter
	itemRepo    repository.WritebackItemRepository
	changeRepo  repository.ChangeRevisionRepository
	maxComments int
	summaryOnly bool
}

const summaryFindingKey = "__summary__"

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

func (o *Orchestrator) Publish(ctx context.Context, req domain.Request, result domain.Result, diff string) (*domain.WritebackResult, error) {
	return o.PublishWithOptions(ctx, req, result, diff, PublishOptions{})
}

func (o *Orchestrator) PublishWithOptions(ctx context.Context, req domain.Request, result domain.Result, diff string, opts PublishOptions) (*domain.WritebackResult, error) {
	adapter, ok := o.adapters[req.Provider]
	if !ok {
		return nil, fmt.Errorf("no writeback adapter for provider %q", req.Provider)
	}

	number := types.ResolvePRNumber(req.Provider, req.PullRequestID, req.MergeRequestIID)
	if number == "" {
		return nil, fmt.Errorf("cannot determine PR/MR number for writeback")
	}

	cr := ChangeRequest{
		Provider:   req.Provider,
		Repository: strings.TrimPrefix(req.Repo, "/"),
		Number:     number,
		BaseSHA:    req.BaseSHA,
		HeadSHA:    req.HeadSHA,
	}
	serviceReviewID := strings.TrimSpace(req.ReviewID)
	existingItems := map[string]wbitem.Item{}
	if serviceReviewID != "" && o.itemRepo != nil {
		items, err := o.itemRepo.ListByReview(ctx, serviceReviewID)
		if err != nil {
			logx.Warn("writeback: failed to load existing item records", "review_id", serviceReviewID, "error", err)
		} else {
			for _, item := range items {
				existingItems[writebackItemKey(item.FindingKey, item.Kind)] = item
			}
		}
	}

	caps, err := adapter.Capabilities(ctx, cr)
	if err != nil {
		return nil, fmt.Errorf("check capabilities: %w", err)
	}

	summaryBody := FormatSummaryBody(result.Summary, result.Findings, o.maxComments)
	if !opts.Force && isPublishedWritebackItem(existingItems[writebackItemKey(summaryFindingKey, "summary")]) {
		summaryBody = ""
	}

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
			if !opts.Force && isPublishedWritebackItem(existingItems[writebackItemKey(vc.Key, "inline")]) {
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

	if strings.TrimSpace(summaryBody) == "" && len(comments) == 0 {
		logx.Info("writeback skipped (already published)")
		return &domain.WritebackResult{Skipped: true}, nil
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

	if wbResult.Stale {
		return o.publishStaleSummary(ctx, adapter, cr, summaryBody, serviceReviewID, wbResult)
	}

	if wbResult.ReviewID == "" && len(wbResult.Comments) == 0 {
		logx.Info("writeback skipped (stale or empty)")
		return &domain.WritebackResult{Skipped: true}, nil
	}

	if serviceReviewID == "" {
		serviceReviewID = wbResult.ReviewID
	}

	published, failed := 0, 0
	if serviceReviewID != "" && o.itemRepo != nil && strings.TrimSpace(summaryBody) != "" {
		if err := o.itemRepo.Create(ctx, &wbitem.Item{
			ReviewID:   serviceReviewID,
			FindingKey: summaryFindingKey,
			Kind:       "summary",
			Status:     "pending",
		}); err != nil {
			logx.Warn("writeback: failed to create summary item record", "error", err)
		}
		if err := o.itemRepo.IncrementAttempts(ctx, serviceReviewID, summaryFindingKey, "summary"); err != nil {
			logx.Warn("writeback: failed to increment summary attempts", "error", err)
		}
		if err := o.itemRepo.UpdateStatus(ctx, serviceReviewID, summaryFindingKey, "summary", "published", wbResult.SummaryID, wbResult.SummaryURL, ""); err != nil {
			logx.Warn("writeback: failed to update summary item status", "error", err)
		} else {
			published++
		}
	}
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
		if o.itemRepo == nil {
			continue
		}
		if err := o.itemRepo.Create(ctx, &wbitem.Item{
			ReviewID:   serviceReviewID,
			FindingKey: cr.Key,
			Kind:       "inline",
			Status:     "pending",
		}); err != nil {
			logx.Warn("writeback: failed to create item record", "key", cr.Key, "error", err)
		}
		if err := o.itemRepo.IncrementAttempts(ctx, serviceReviewID, cr.Key, "inline"); err != nil {
			logx.Warn("writeback: failed to increment attempts", "key", cr.Key, "error", err)
		}
		if err := o.itemRepo.UpdateStatus(ctx, serviceReviewID, cr.Key, "inline", status, cr.ExternalID, cr.URL, cr.Error); err != nil {
			logx.Warn("writeback: failed to update item status", "key", cr.Key, "error", err)
		}
	}

	logx.Info("writeback completed",
		"provider", req.Provider,
		"repo", req.Repo,
		"number", number,
		"comments", len(comments),
		"published", published,
		"failed", failed,
	)
	return &domain.WritebackResult{Published: published, Failed: failed}, nil
}

func writebackItemKey(findingKey, kind string) string {
	return kind + "\x00" + findingKey
}

// publishStaleSummary 在检测到 head SHA 已变（评审过期）后，以当前 head 降级发布一份
// 仅含摘要的回写。行内评论依赖具体行号，过期后无法可靠锚定；而摘要（分析/风险/建议）
// 不依赖行号，始终可安全发布，故降级为 summary-only，并在顶部追加过期横幅，提示作者
// 对最新提交重新触发评审以获取行内评论。
func (o *Orchestrator) publishStaleSummary(ctx context.Context, adapter Adapter, cr ChangeRequest, summaryBody, serviceReviewID string, stale Result) (*domain.WritebackResult, error) {
	expected := stale.ExpectedSHA
	actual := stale.ActualSHA
	logx.Info("writeback degraded (stale): publishing summary-only on current head",
		"expected", expected, "actual", actual)

	body := strings.TrimSpace(summaryBody)
	if body == "" {
		body = "（评审摘要为空）"
	}
	degradedRev := Review{
		ChangeRequest: ChangeRequest{
			Provider:   cr.Provider,
			Repository: cr.Repository,
			Number:     cr.Number,
			BaseSHA:    cr.BaseSHA,
			HeadSHA:    actual,
		},
		Summary:  FormatStaleBanner(expected, actual) + "\n\n" + body,
		Event:    EventComment,
		Comments: nil,
	}

	degraded, err := adapter.Publish(ctx, degradedRev)
	if err != nil {
		return nil, fmt.Errorf("publish stale summary: %w", err)
	}

	published := 0
	if serviceReviewID == "" {
		serviceReviewID = degraded.ReviewID
	}
	if serviceReviewID != "" && o.itemRepo != nil && (degraded.ReviewID != "" || degraded.SummaryID != "") {
		if err := o.itemRepo.Create(ctx, &wbitem.Item{
			ReviewID:   serviceReviewID,
			FindingKey: summaryFindingKey,
			Kind:       "summary",
			Status:     "pending",
		}); err != nil {
			logx.Warn("writeback: failed to create stale summary item record", "error", err)
		}
		if err := o.itemRepo.IncrementAttempts(ctx, serviceReviewID, summaryFindingKey, "summary"); err != nil {
			logx.Warn("writeback: failed to increment stale summary attempts", "error", err)
		}
		if err := o.itemRepo.UpdateStatus(ctx, serviceReviewID, summaryFindingKey, "summary", "downgraded", degraded.SummaryID, degraded.SummaryURL, ""); err != nil {
			logx.Warn("writeback: failed to update stale summary item status", "error", err)
		} else {
			published = 1
		}
	} else if degraded.ReviewID != "" || degraded.SummaryID != "" {
		published = 1
	}

	return &domain.WritebackResult{
		Published:   published,
		Stale:       true,
		ExpectedSHA: expected,
		ActualSHA:   actual,
	}, nil
}

func isPublishedWritebackItem(item wbitem.Item) bool {
	return item.Status == "published" || item.Status == "downgraded"
}

func (o *Orchestrator) Status(ctx context.Context, reviewID string) (*PublishStatus, error) {
	items, err := o.itemRepo.ListByReview(ctx, reviewID)
	if err != nil {
		return nil, err
	}
	status := &PublishStatus{
		ReviewID: reviewID,
		Items:    make([]PublishStatusItem, 0, len(items)),
	}
	for _, item := range items {
		status.Total++
		switch item.Status {
		case "published":
			status.Published++
		case "failed":
			status.Failed++
		case "downgraded":
			status.Downgraded++
		default:
			status.Pending++
		}
		status.Items = append(status.Items, PublishStatusItem{
			FindingKey: item.FindingKey,
			Kind:       item.Kind,
			Status:     item.Status,
			Attempts:   item.Attempts,
			ExternalID: item.ProviderExternalID,
			URL:        item.ProviderURL,
			LastError:  item.LastError,
		})
	}
	return status, nil
}

func findSeverity(findings []domain.Finding, i int) string {
	if i < len(findings) {
		return findings[i].Severity
	}
	return domain.SeverityInfo
}

func findTitle(findings []domain.Finding, i int) string {
	if i < len(findings) {
		return findings[i].Title
	}
	return ""
}

func findExistingCode(findings []domain.Finding, i int) string {
	if i < len(findings) {
		return findings[i].ExistingCode
	}
	return ""
}
