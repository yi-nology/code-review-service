package writeback

import (
	"context"
	"errors"
	"fmt"

	gitprovider "github.com/yi-nology/code-review-service/internal/service/provider"

	"github.com/cloudwego/hertz/pkg/common/hlog"
	sdk "github.com/yi-nology/git-platform-sdk/provider"
)

type GiteaAdapter struct {
	svc *gitprovider.GitProviderService
}

func NewGiteaAdapter(svc *gitprovider.GitProviderService) *GiteaAdapter {
	return &GiteaAdapter{svc: svc}
}

func (a *GiteaAdapter) Capabilities(ctx context.Context, cr ChangeRequest) (Capabilities, error) {
	return Capabilities{
		BatchReview:       true,
		MultiLineComments: false,
		Suggestions:       false,
		UpdateSummary:     false,
	}, nil
}

func (a *GiteaAdapter) Publish(ctx context.Context, rev Review) (Result, error) {
	owner, repo, err := gitprovider.ParseRepoPath(rev.ChangeRequest.Repository)
	if err != nil {
		return Result{}, fmt.Errorf("parse repo path: %w", err)
	}

	p, err := a.svc.GetProvider("gitea")
	if err != nil {
		return Result{}, err
	}

	cr, err := p.GetCR(ctx, owner, repo, rev.ChangeRequest.Number)
	if err != nil {
		hlog.Warn("failed to get PR for stale check", "error", err)
	} else if cr != nil && cr.HeadSHA != "" && cr.HeadSHA != rev.ChangeRequest.HeadSHA {
		hlog.Warn("stale review, head SHA changed",
			"expected", rev.ChangeRequest.HeadSHA,
			"actual", cr.HeadSHA,
		)
		return Result{}, nil
	}

	comments := make([]sdk.ReviewComment, 0, len(rev.Comments))
	for _, c := range rev.Comments {
		rc := sdk.ReviewComment{
			Path: c.Path,
			Body: c.Body,
		}
		if c.Range.Side == SideOld {
			rc.Side = "LEFT"
			rc.Line = c.Range.Start
		} else {
			rc.Line = c.Range.Start
		}
		comments = append(comments, rc)
	}

	event := "COMMENT"
	if rev.Event == EventRequestChanges {
		event = "REQUEST_CHANGES"
	}

	reviewResult, err := p.CreateReview(ctx, owner, repo, rev.ChangeRequest.Number, sdk.CreateReviewOptions{
		CommitID: rev.ChangeRequest.HeadSHA,
		Event:    event,
		Body:     rev.Summary,
		Comments: comments,
	})
	if err != nil {
		hlog.Warn("create review failed, falling back to note", "error", err)
		summaryID, noteErr := p.CreateNote(ctx, owner, repo, rev.ChangeRequest.Number, rev.Summary)
		if noteErr != nil {
			return Result{}, fmt.Errorf("create review and fallback note failed: %w", errors.Join(err, noteErr))
		}
		return Result{SummaryID: summaryID}, nil
	}

	result := Result{
		ReviewID:  reviewResult.ID,
		SummaryID: reviewResult.ID,
	}
	if reviewResult.HTMLURL != "" {
		result.SummaryURL = reviewResult.HTMLURL
	}

	for _, c := range rev.Comments {
		result.Comments = append(result.Comments, CommentResult{
			Key:    c.Key,
			Inline: true,
		})
	}

	return result, nil
}
