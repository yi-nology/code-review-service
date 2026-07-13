package notification

import (
	"context"
	"fmt"

	"github.com/yi-nology/code-review-service/internal/domain/notification"
	"github.com/yi-nology/code-review-service/internal/repository"
)

// InAppChannel 把评审事件写成站内通知（notifications 表），供前端通知中心展示。
type InAppChannel struct {
	repo repository.NotificationRepository
}

func NewInAppChannel(repo repository.NotificationRepository) *InAppChannel {
	return &InAppChannel{repo: repo}
}

func (c *InAppChannel) Name() string { return "inapp" }

func (c *InAppChannel) Send(ctx context.Context, event notification.Event) error {
	title, body := formatEvent(event)
	item := &notification.Item{
		ReviewID: event.ReviewID,
		Kind:     event.Kind,
		Title:    title,
		Body:     body,
		Provider: event.Provider,
		Repo:     event.Repo,
	}
	return c.repo.Create(ctx, item)
}

// formatEvent 把评审事件格式化为通知标题和正文。
// 标题简洁（通知列表展示），正文含关键信息（门禁/评分/失败原因）。
func formatEvent(event notification.Event) (title, body string) {
	ref := event.Ref
	if ref == "" {
		ref = fmt.Sprintf("%s/%s", event.Provider, event.Repo)
	}
	switch event.Kind {
	case notification.KindFailed:
		title = fmt.Sprintf("评审失败 · %s", ref)
		detail := event.Stage
		if event.Error != "" {
			detail = event.Error
			if len(detail) > 200 {
				detail = detail[:200] + "…"
			}
		}
		body = fmt.Sprintf("评审 %s 在「%s」阶段失败：%s", event.ReviewID, event.Stage, detail)
	case notification.KindGateFailed:
		title = fmt.Sprintf("门禁未通过 · %s", ref)
		body = fmt.Sprintf("评审 %s 完成，但质量门禁未通过。发现 %d 个问题，评分 %s。",
			event.ReviewID, event.Findings, gradeText(event.Grade))
		if event.Summary != "" {
			s := event.Summary
			if len(s) > 300 {
				s = s[:300] + "…"
			}
			body += "\n\n" + s
		}
	default: // KindCompleted
		title = fmt.Sprintf("评审完成 · %s", ref)
		body = fmt.Sprintf("评审 %s 完成，评分 %s，发现 %d 个问题。",
			event.ReviewID, gradeText(event.Grade), event.Findings)
	}
	return title, body
}

func gradeText(grade string) string {
	if grade == "" {
		return "—"
	}
	return grade
}
