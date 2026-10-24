package gerrit

import (
	"context"
	"fmt"

	gogerrit "github.com/andygrunwald/go-gerrit"
	"github.com/oekit/logx"

	"github.com/oekit/pkg/ptrx"
)

// ReviewInput 描述一次 Gerrit 评审请求。
type ReviewInput struct {
	ChangeNumber string          // Gerrit change 的数字编号
	RevisionID   string          // patchset revision SHA，或 "current"
	CommitSHA    string          // 用于 stale check 的期望 head SHA
	Message      string          // 摘要评论（change message）
	Comments     []InlineComment // 行内评论列表
	Label        *LabelVote      // 可选的 label 投票
}

// InlineComment 描述一条行内评论。
type InlineComment struct {
	Path       string // 文件路径（相对于 repo root）
	Line       int    // 起始行号
	EndLine    int    // 结束行号（0 表示单行）
	Side       string // "REVISION"（新代码）或 "PARENT"（旧代码）
	Message    string // 评论内容
	Unresolved bool   // 是否标记为未解决
}

// LabelVote 描述一次 label 投票。
type LabelVote struct {
	Label string // label 名称，如 "Code-Review"
	Value int    // 投票值，-2 到 +2
}

// SetReview 发表一次 Gerrit 评审，包含摘要评论、行内评论和可选的 label 投票。
//
// 内部映射：
//   - ReviewInput.Message → gogerrit.ReviewInput.Message
//   - InlineComment → gogerrit.CommentInput（key 为文件路径）
//   - LabelVote → gogerrit.ReviewInput.Labels
func (c *Client) SetReview(ctx context.Context, input ReviewInput) (*gogerrit.ReviewResult, error) {
	revisionID := input.RevisionID
	if revisionID == "" {
		revisionID = "current"
	}

	reviewInput := &gogerrit.ReviewInput{
		Message: input.Message,
	}

	// 映射 label 投票
	if input.Label != nil {
		reviewInput.Labels = map[string]int{
			input.Label.Label: input.Label.Value,
		}
	}

	// 映射行内评论：Gerrit 的 Comments 是 map[filePath][]CommentInput
	if len(input.Comments) > 0 {
		reviewInput.Comments = make(map[string][]gogerrit.CommentInput)
		for _, c := range input.Comments {
			ci := gogerrit.CommentInput{
				Path:       c.Path,
				Message:    c.Message,
				Unresolved: ptrx.Of(c.Unresolved),
			}

			// 行号映射
			if c.Line > 0 {
				if c.EndLine > c.Line {
					// 多行评论使用 Range
					ci.Line = c.Line
					ci.Range = &gogerrit.CommentRange{
						StartLine: c.Line,
						EndLine:   c.EndLine,
					}
				} else {
					// 单行评论
					ci.Line = c.Line
				}
			}

			// Side 映射：REVISION/PARENT
			if c.Side != "" {
				ci.Side = c.Side
			}

			reviewInput.Comments[c.Path] = append(reviewInput.Comments[c.Path], ci)
		}
	}

	logx.Info("gerrit: posting review",
		"change", input.ChangeNumber,
		"revision", revisionID,
		"comments", len(input.Comments),
		"has_label", input.Label != nil,
	)

	result, _, err := c.raw.Changes.SetReview(ctx, input.ChangeNumber, revisionID, reviewInput)
	if err != nil {
		return nil, fmt.Errorf("gerrit: set review on change %s: %w", input.ChangeNumber, err)
	}

	return result, nil
}

// SetReviewMessage 仅发表一条 change message（不含行内评论和 label）。
// 用于降级场景：当 SetReview 失败时，降级为只发摘要消息。
func (c *Client) SetReviewMessage(ctx context.Context, changeNumber, message string) error {
	_, err := c.SetReview(ctx, ReviewInput{
		ChangeNumber: changeNumber,
		RevisionID:   "current",
		Message:      message,
	})
	return err
}

// SetLabelVote 仅设置 label 投票（不含评论）。
func (c *Client) SetLabelVote(ctx context.Context, changeNumber, label string, value int) error {
	_, err := c.SetReview(ctx, ReviewInput{
		ChangeNumber: changeNumber,
		RevisionID:   "current",
		Label: &LabelVote{
			Label: label,
			Value: value,
		},
	})
	return err
}

