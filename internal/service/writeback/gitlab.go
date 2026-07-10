package writeback

import (
	"context"
	"fmt"

	gitprovider "github.com/yi-nology/code-review-service/internal/service/provider"

	"github.com/oekit/logx"
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
		logx.Warn("failed to get MR for stale check", "error", err)
	} else if cr != nil && cr.HeadSHA != "" && cr.HeadSHA != rev.ChangeRequest.HeadSHA {
		logx.Warn("stale review, head SHA changed",
			"expected", rev.ChangeRequest.HeadSHA,
			"actual", cr.HeadSHA,
		)
		return Result{Stale: true, ExpectedSHA: rev.ChangeRequest.HeadSHA, ActualSHA: cr.HeadSHA}, nil
	}

	comments := make([]sdk.ReviewComment, 0, len(rev.Comments))
	for _, c := range rev.Comments {
		rc := sdk.ReviewComment{
			Path:      c.Path,
			Body:      c.Body,
			Line:      c.Range.Start,
			StartLine: c.Range.Start,
			EndLine:   c.Range.End,
		}
		if c.Range.Side == SideOld {
			rc.Side = "LEFT"
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

	for i, c := range rev.Comments {
		cr := CommentResult{
			Key:    c.Key,
			Inline: true,
		}
		if i < len(reviewResult.Comments) {
			rc := reviewResult.Comments[i]
			cr.ExternalID = rc.ExternalID
			if rc.Error != "" {
				cr.Inline = false
				cr.Error = rc.Error
				logx.Warn("gitlab inline comment failed",
					"path", c.Path,
					"line", c.Range.Start,
					"error", rc.Error,
				)
			}
		}
		result.Comments = append(result.Comments, cr)
	}

	return result, nil
}
