package writeback

import (
	"context"
	"fmt"

	gitprovider "review-service/internal/service/provider"

	"github.com/cloudwego/hertz/pkg/common/hlog"
	sdk "github.com/yi-nology/git-platform-sdk/provider"
)

type GitLabAdapter struct {
	svc *gitprovider.GitProviderService
}

func NewGitLabAdapter(svc *gitprovider.GitProviderService) *GitLabAdapter {
	return &GitLabAdapter{svc: svc}
}

func (a *GitLabAdapter) Capabilities(ctx context.Context, cr ChangeRequest) (Capabilities, error) {
	return Capabilities{
		BatchReview:       true,
		MultiLineComments: true,
		Suggestions:       true,
		UpdateSummary:     false,
	}, nil
}

func (a *GitLabAdapter) Publish(ctx context.Context, rev Review) (Result, error) {
	owner, repo, err := gitprovider.ParseRepoPath(rev.ChangeRequest.Repository)
	if err != nil {
		return Result{}, fmt.Errorf("parse repo path: %w", err)
	}

	p, err := a.svc.GetProvider("gitlab")
	if err != nil {
		return Result{}, err
	}

	cr, err := p.GetCR(ctx, owner, repo, rev.ChangeRequest.Number)
	if err != nil {
		hlog.Warn("failed to get MR for stale check", "error", err)
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
		return Result{}, fmt.Errorf("create review: %w", err)
	}

	result := Result{
		ReviewID:  reviewResult.ID,
		SummaryID: reviewResult.ID,
	}

	for i, c := range rev.Comments {
		result.Comments = append(result.Comments, CommentResult{
			Key:    c.Key,
			Inline: true,
		})
		_ = i
	}

	return result, nil
}
