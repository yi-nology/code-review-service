package writeback

import (
	"context"
	"fmt"

	gitprovider "review-service/internal/service/provider"

	"github.com/cloudwego/hertz/pkg/common/hlog"
	sdk "github.com/yi-nology/git-platform-sdk/provider"
)

type GitHubAdapter struct {
	svc *gitprovider.GitProviderService
}

func NewGitHubAdapter(svc *gitprovider.GitProviderService) *GitHubAdapter {
	return &GitHubAdapter{svc: svc}
}

func (a *GitHubAdapter) Capabilities(ctx context.Context, cr ChangeRequest) (Capabilities, error) {
	return Capabilities{
		BatchReview:       true,
		MultiLineComments: true,
		Suggestions:       true,
		UpdateSummary:     false,
	}, nil
}

func (a *GitHubAdapter) Publish(ctx context.Context, rev Review) (Result, error) {
	owner, repo, err := gitprovider.ParseRepoPath(rev.ChangeRequest.Repository)
	if err != nil {
		return Result{}, fmt.Errorf("parse repo path: %w", err)
	}

	p, err := a.svc.GetProvider("github")
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
		if c.Range.Start > 0 {
			if c.Range.End > c.Range.Start {
				rc.StartLine = c.Range.Start
				rc.EndLine = c.Range.End
				rc.Side = "RIGHT"
				if c.Range.Side == SideOld {
					rc.Side = "LEFT"
				}
			} else {
				rc.Line = c.Range.Start
				rc.Side = "RIGHT"
				if c.Range.Side == SideOld {
					rc.Side = "LEFT"
				}
			}
		}
		comments = append(comments, rc)
	}

	event := "COMMENT"
	switch rev.Event {
	case EventRequestChanges:
		event = "REQUEST_CHANGES"
	case EventApprove:
		event = "APPROVE"
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
