package writeback

import (
	"context"
	"errors"
	"fmt"

	"github.com/oekit/logx"
	gerrit "github.com/yi-nology/code-review-service/internal/service/gerrit"
)

// gerritAdapter 实现 writeback.Adapter 接口，使用 go-gerrit 直接调用
// Gerrit REST API 进行评审回写。
//
// 与其他平台的 platformAdapter 不同，Gerrit 不走 go-git-platform 的
// ReviewManager 接口，而是通过独立的 gerrit.Client 发表评审。
// Gerrit 的评审模型也不同：使用 label 投票（-2~+2）代替 commit status。
type gerritAdapter struct {
	// get 返回当前生效的 Gerrit 客户端。持 getter 而非固定 *Client：
	// 适配器在装配期就构造出来了，若把客户端捕获进去，之后在 web console
	// 改平台配置（换 token / 换地址）也不会生效——只能重启进程。
	get func() *gerrit.Client
}

// NewGerritAdapter 构造 Gerrit 写回适配器。get 为 nil 时适配器不可用。
func NewGerritAdapter(get func() *gerrit.Client) Adapter {
	return &gerritAdapter{get: get}
}

// client 取当前客户端；未配置时返回错误而不是 nil 指针。
func (a *gerritAdapter) client() (*gerrit.Client, error) {
	if a.get == nil {
		return nil, fmt.Errorf("gerrit: 客户端未接入")
	}
	cl := a.get()
	if cl == nil {
		return nil, fmt.Errorf("gerrit: 客户端未配置（请在「配置 → Git 平台」配置 Gerrit 平台）")
	}
	return cl, nil
}

func (a *gerritAdapter) Capabilities(_ context.Context, _ ChangeRequest) (Capabilities, error) {
	return Capabilities{
		BatchReview:       true,
		MultiLineComments: true,
		Suggestions:       false, // Gerrit 不支持 GitHub-style suggestion 语法
		UpdateSummary:     false,
	}, nil
}

func (a *gerritAdapter) Publish(ctx context.Context, rev Review) (Result, error) {
	client, err := a.client()
	if err != nil {
		return Result{}, err
	}

	// Gerrit 的 Repository 格式为 "/project"，需要提取 project name。
	project := gerrit.ExtractProjectFromPath(rev.ChangeRequest.Repository)
	if project == "" {
		return Result{}, fmt.Errorf("gerrit: empty project in repository path %q", rev.ChangeRequest.Repository)
	}

	// Stale check：比对当前 Gerrit 端的 head SHA
	headSHA, err := client.GetHeadSHA(ctx, rev.ChangeRequest.Number)
	if err != nil {
		logx.Warn("gerrit: failed to get head SHA for stale check",
			"change", rev.ChangeRequest.Number,
			"error", err,
		)
	} else if headSHA != "" && headSHA != rev.ChangeRequest.HeadSHA {
		logx.Warn("gerrit: stale review, head SHA changed",
			"change", rev.ChangeRequest.Number,
			"expected", rev.ChangeRequest.HeadSHA,
			"actual", headSHA,
		)
		return Result{
			Stale:       true,
			ExpectedSHA: rev.ChangeRequest.HeadSHA,
			ActualSHA:   headSHA,
		}, nil
	}

	// 构建行内评论
	comments := buildGerritComments(rev.Comments)

	// 确定 label 投票
	label := mapGerritLabelVote(rev.Event)

	reviewResult, err := client.SetReview(ctx, gerrit.ReviewInput{
		ChangeNumber: rev.ChangeRequest.Number,
		RevisionID:   rev.ChangeRequest.HeadSHA,
		CommitSHA:    rev.ChangeRequest.HeadSHA,
		Message:      rev.Summary,
		Comments:     comments,
		Label:        label,
	})
	if err != nil {
		// 降级：仅发表摘要消息（不含行内评论和 label）
		logx.Warn("gerrit: set review failed, falling back to message-only",
			"change", rev.ChangeRequest.Number,
			"error", err,
		)
		msgErr := client.SetReviewMessage(ctx, rev.ChangeRequest.Number, rev.Summary)
		if msgErr != nil {
			return Result{}, fmt.Errorf("gerrit: set review and fallback message both failed: %w", errors.Join(err, msgErr))
		}
		return Result{SummaryID: "message-only"}, nil
	}

	result := Result{
		ReviewID:  "gerrit-review",
		SummaryID: "gerrit-review",
	}

	// Gerrit 的 ReviewResult 不像 GitHub 那样返回每条评论的 ID，
	// 但能确认评论是否成功。
	for _, c := range rev.Comments {
		result.Comments = append(result.Comments, CommentResult{
			Key:    c.Key,
			Inline: true,
		})
	}

	if reviewResult != nil && reviewResult.Labels != nil {
		logx.Info("gerrit: review posted successfully",
			"change", rev.ChangeRequest.Number,
			"labels", reviewResult.Labels,
		)
	}

	return result, nil
}

// buildGerritComments 将 writeback.Comment 映射为 gerrit.InlineComment。
// Gerrit 的行号映射：
//   - Line: 起始行号
//   - EndLine: 结束行号（多行评论）
//   - Side: "REVISION"（新代码，默认）/ "PARENT"（旧代码）
func buildGerritComments(comments []Comment) []gerrit.InlineComment {
	out := make([]gerrit.InlineComment, 0, len(comments))
	for _, c := range comments {
		gc := gerrit.InlineComment{
			Path:       c.Path,
			Message:    c.Body,
			Unresolved: true, // 默认标记为未解决
		}

		if c.Range.Start > 0 {
			gc.Line = c.Range.Start
			if c.Range.End > c.Range.Start {
				gc.EndLine = c.Range.End
			}
		}

		// Side 映射
		if c.Range.Side == SideOld {
			gc.Side = "PARENT"
		} else {
			gc.Side = "REVISION"
		}

		out = append(out, gc)
	}
	return out
}

// mapGerritLabelVote 根据评审事件类型确定 Gerrit label 投票。
//
//   - EventApprove → Code-Review: +1
//   - EventRequestChanges → Code-Review: -1
//   - 其他（EventComment / EventReview）→ 不投票
func mapGerritLabelVote(event Event) *gerrit.LabelVote {
	switch event {
	case EventApprove:
		return &gerrit.LabelVote{Label: "Code-Review", Value: 1}
	case EventRequestChanges:
		return &gerrit.LabelVote{Label: "Code-Review", Value: -1}
	default:
		return nil
	}
}
