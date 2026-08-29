package writeback

import (
	"context"
	"errors"
	"fmt"

	gitprovider "github.com/yi-nology/code-review-service/internal/service/provider"

	"github.com/oekit/logx"
	sdk "github.com/yi-nology/git-platform-sdk/provider"
)

// platformAdapter 是各平台写回适配器的通用实现。各平台（github/gitlab/
// gitea/forgejo/gitcode/tencent_code）的 Publish 流程一致：stale 检查 →
// 行内评论构建 → CreateReview（失败按需降级 note）→ 结果映射，平台差异
// 全部收敛到 AdapterOptions：
//   - NoteFallback：CreateReview 整体失败时，降级为一条 note 发布摘要。
//   - InlineFallback：review 成功但个别行内评论失败时，按下标回读
//     reviewResult.Comments 并逐条降级为 note（gitlab 语义）。
//
// 行内评论的行号映射按平台保留历史行为，见 buildComments。
type platformAdapter struct {
	svc  *gitprovider.GitProviderService
	opts AdapterOptions
}

// AdapterOptions 描述平台差异，见 platformAdapter 注释。
type AdapterOptions struct {
	Platform       string
	NoteFallback   bool
	InlineFallback bool
}

// NewPlatformAdapter 构造通用平台写回适配器。
func NewPlatformAdapter(svc *gitprovider.GitProviderService, opts AdapterOptions) Adapter {
	return &platformAdapter{svc: svc, opts: opts}
}

func (a *platformAdapter) Capabilities(ctx context.Context, cr ChangeRequest) (Capabilities, error) {
	return Capabilities{
		BatchReview:       true,
		MultiLineComments: true,
		Suggestions:       true,
		UpdateSummary:     false,
	}, nil
}

func (a *platformAdapter) Publish(ctx context.Context, rev Review) (Result, error) {
	owner, repo, err := gitprovider.ParseRepoPath(rev.ChangeRequest.Repository)
	if err != nil {
		return Result{}, fmt.Errorf("parse repo path: %w", err)
	}

	p, err := a.svc.GetProvider(a.opts.Platform)
	if err != nil {
		return Result{}, err
	}

	cr, err := p.GetCR(ctx, owner, repo, rev.ChangeRequest.Number)
	if err != nil {
		logx.Warn("failed to get PR/MR for stale check", "error", err)
	} else if cr != nil && cr.HeadSHA != "" && cr.HeadSHA != rev.ChangeRequest.HeadSHA {
		logx.Warn("stale review, head SHA changed",
			"expected", rev.ChangeRequest.HeadSHA,
			"actual", cr.HeadSHA,
		)
		return Result{Stale: true, ExpectedSHA: rev.ChangeRequest.HeadSHA, ActualSHA: cr.HeadSHA}, nil
	}

	// SDK v0.48 起 CreateReview 移到可选 ReviewManager 能力接口，需断言。
	rm, ok := p.(sdk.ReviewManager)
	if !ok {
		return Result{}, fmt.Errorf("%s provider does not support reviews", a.opts.Platform)
	}

	event := "COMMENT"
	switch rev.Event {
	case EventRequestChanges:
		event = "REQUEST_CHANGES"
	case EventApprove:
		event = "APPROVE"
	}

	reviewResult, err := rm.CreateReview(ctx, owner, repo, rev.ChangeRequest.Number, sdk.CreateReviewOptions{
		CommitID: rev.ChangeRequest.HeadSHA,
		Event:    event,
		Body:     rev.Summary,
		Comments: buildComments(a.opts.Platform, rev.Comments),
	})
	if err != nil {
		if !a.opts.NoteFallback {
			return Result{}, fmt.Errorf("create review: %w", err)
		}
		logx.Warn("create review failed, falling back to note", "error", err)
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

	if a.opts.InlineFallback {
		result.Comments = a.inlineResultsWithFallback(ctx, p, owner, repo, rev, reviewResult)
	} else {
		for _, c := range rev.Comments {
			result.Comments = append(result.Comments, CommentResult{
				Key:    c.Key,
				Inline: true,
			})
		}
	}

	return result, nil
}

// inlineResultsWithFallback 按 gitlab 语义映射评论结果：从 reviewResult.Comments
// 按下标取 ExternalID；发布失败的评论降级为 MR note（降级后 Inline=false，
// 锚点信息存 URL 语义的 ExternalID）。
func (a *platformAdapter) inlineResultsWithFallback(ctx context.Context, p sdk.Provider, owner, repo string, rev Review, reviewResult *sdk.ReviewResult) []CommentResult {
	results := make([]CommentResult, 0, len(rev.Comments))
	for i, c := range rev.Comments {
		cr := CommentResult{
			Key:    c.Key,
			Inline: true,
		}
		if i < len(reviewResult.Comments) {
			rc := reviewResult.Comments[i]
			cr.ExternalID = rc.ExternalID
			if rc.Error != "" {
				noteBody := fmt.Sprintf("**📄 `%s:%d`**\n\n%s", c.Path, c.Range.Start, c.Body)
				noteID, noteErr := p.CreateNote(ctx, owner, repo, rev.ChangeRequest.Number, noteBody)
				if noteErr != nil {
					cr.Inline = false
					cr.Error = rc.Error
					logx.Warn("inline comment failed (note fallback also failed)",
						"path", c.Path,
						"line", c.Range.Start,
						"error", rc.Error,
						"note_error", noteErr,
					)
				} else {
					cr.ExternalID = noteID
					cr.Inline = false
					logx.Info("inline comment fell back to note",
						"path", c.Path,
						"line", c.Range.Start,
					)
				}
			}
		}
		results = append(results, cr)
	}
	return results
}

// buildComments 按平台映射行内评论行号，保留各平台历史行为：
//   - github：多行用 StartLine/EndLine，单行用 Line；Start==0 不锚定行号。
//   - gitlab：Line/StartLine/EndLine 全量下发。
//   - 其余（gitea 家族/gitcode）：仅 Line。
func buildComments(platform string, comments []Comment) []sdk.ReviewComment {
	out := make([]sdk.ReviewComment, 0, len(comments))
	for _, c := range comments {
		rc := sdk.ReviewComment{
			Path: c.Path,
			Body: c.Body,
		}
		switch platform {
		case "github":
			if c.Range.Start > 0 {
				side := "RIGHT"
				if c.Range.Side == SideOld {
					side = "LEFT"
				}
				if c.Range.End > c.Range.Start {
					rc.StartLine = c.Range.Start
					rc.EndLine = c.Range.End
					rc.Side = side
				} else {
					rc.Line = c.Range.Start
					rc.Side = side
				}
			}
		case "gitlab":
			rc.Line = c.Range.Start
			rc.StartLine = c.Range.Start
			rc.EndLine = c.Range.End
			if c.Range.Side == SideOld {
				rc.Side = "LEFT"
			}
		default:
			rc.Line = c.Range.Start
			if c.Range.Side == SideOld {
				rc.Side = "LEFT"
			} else {
				rc.Side = "RIGHT"
			}
		}
		out = append(out, rc)
	}
	return out
}
