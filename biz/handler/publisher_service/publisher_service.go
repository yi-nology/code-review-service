package publisher_service

import (
	"context"
	"strings"

	"github.com/yi-nology/code-review-service/biz/handler/common"
	modelPublisher "github.com/yi-nology/code-review-service/biz/model/publisher"
	"github.com/yi-nology/code-review-service/internal/service/provider"
	"github.com/yi-nology/code-review-service/internal/service/review"
	"github.com/yi-nology/code-review-service/internal/service/types"
	"github.com/yi-nology/code-review-service/internal/service/writeback"

	"github.com/oekit/logx"
	"github.com/cloudwego/hertz/pkg/app"
)

var handlerInst *PublisherHandler //nolint:unused // initialized by InitHandler, used by route handlers

// reviewGetter 是 publisher 需要的"按 ID 取评审"能力（最小接口，避免耦合具体实现）。
type reviewGetter interface {
	Get(ctx context.Context, reviewID string) (review.Record, bool, error)
}

type PublisherHandler struct {
	writeback   *writeback.Orchestrator
	reviewSvc   reviewGetter
	providerSvc *provider.GitProviderService
}

// InitHandler 注入 writeback 编排器、评审服务、Git 平台服务。
func InitHandler(writeback *writeback.Orchestrator, reviewSvc reviewGetter, providerSvc *provider.GitProviderService) {
	handlerInst = &PublisherHandler{
		writeback:   writeback,
		reviewSvc:   reviewSvc,
		providerSvc: providerSvc,
	}
}

func PublishReview(ctx context.Context, c *app.RequestContext) {
	req, ok := common.Bind[modelPublisher.PublishReviewReq](c)
	if !ok {
		return
	}

	if err := publishReview(ctx, req.ID, req.SummaryOnly != nil && *req.SummaryOnly); err != nil {
		common.Fail(c, err)
		return
	}

	common.Success(c, &modelPublisher.PublishReviewResp{
		Message:  "review published successfully",
		ReviewID: req.ID,
	})
}

func PublishStatus(ctx context.Context, c *app.RequestContext) {
	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		common.Fail(c, common.BadRequest("review id 不能为空"))
		return
	}
	status, err := handlerInst.writeback.Status(ctx, id)
	if err != nil {
		common.Fail(c, err)
		return
	}
	common.Success(c, status)
}

func RetryPublish(ctx context.Context, c *app.RequestContext) {
	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		common.Fail(c, common.BadRequest("review id 不能为空"))
		return
	}
	status, err := handlerInst.writeback.Status(ctx, id)
	if err != nil {
		common.Fail(c, err)
		return
	}
	if status.Failed == 0 && status.Pending == 0 {
		common.Success(c, map[string]any{"message": "no retryable writeback items", "review_id": id})
		return
	}
	if err := publishReview(ctx, id, false); err != nil {
		common.Fail(c, err)
		return
	}
	common.Success(c, map[string]any{"message": "retry published", "review_id": id})
}

func publishReview(ctx context.Context, id string, summaryOnly bool) error {
	record, found, err := handlerInst.reviewSvc.Get(ctx, id)
	if err != nil || !found {
		return common.NotFound("review not found")
	}

	findings := record.Response.Findings
	result := review.Result{
		Summary:  record.Response.Summary,
		Findings: findings,
	}

	// 获取 diff 用于行内评论定位（summary_only 或 diff 为空时 orchestrator 降级为 summary-only）
	diff := ""
	if !summaryOnly && handlerInst.providerSvc != nil {
		owner, repoName, perr := provider.ParseRepoPath(record.Request.Repo)
		number := types.ResolvePRNumber(record.Request.Provider, record.Request.PullRequestID, record.Request.MergeRequestIID)
		if perr == nil && number > 0 {
			if d, derr := handlerInst.providerSvc.GetCRDiffString(ctx, record.Request.Provider, owner, repoName, number); derr == nil {
				diff = d
			} else {
				logx.Warn("failed to fetch diff for publish, falling back to summary-only", "error", derr)
			}
		}
	}

	publishReq := record.Request
	publishReq.ReviewID = record.ID
	if _, err := handlerInst.writeback.Publish(ctx, publishReq, result, diff); err != nil {
		return err
	}
	return nil
}

func PreviewComment(ctx context.Context, c *app.RequestContext) {
	req, ok := common.Bind[modelPublisher.PreviewCommentReq](c)
	if !ok {
		return
	}

	record, found, err := handlerInst.reviewSvc.Get(ctx, req.ID)
	if err != nil || !found {
		common.Fail(c, common.NotFound("review not found"))
		return
	}

	findings := record.Response.Findings
	summary := record.Response.Summary

	common.Success(c, &modelPublisher.PreviewCommentResp{
		ReviewID: req.ID,
		Summary:  summary,
		Findings: formatFindingsPreview(findings),
		Total:    int64(len(findings)),
	})
}

func UpdatePublishedComment(ctx context.Context, c *app.RequestContext) {
	req, ok := common.Bind[modelPublisher.UpdatePublishedCommentReq](c)
	if !ok {
		return
	}

	if err := publishReview(ctx, req.ID, false); err != nil {
		common.Fail(c, err)
		return
	}

	common.Success(c, &modelPublisher.UpdatePublishedCommentResp{
		Message:  "published comment refreshed successfully",
		ReviewID: req.ID,
	})
}

// formatFindingsPreview 把 findings 摘要为可读字符串供预览。
func formatFindingsPreview(findings []review.Finding) string {
	if len(findings) == 0 {
		return ""
	}
	var sb strings.Builder
	limit := len(findings)
	if limit > 20 {
		limit = 20
	}
	for i := 0; i < limit; i++ {
		f := findings[i]
		if f.Title != "" {
			sb.WriteString("- [")
			sb.WriteString(string(f.Severity))
			sb.WriteString("] ")
			sb.WriteString(f.Path)
			sb.WriteString(": ")
			sb.WriteString(f.Title)
			sb.WriteString("\n")
		}
	}
	if len(findings) > limit {
		sb.WriteString("...\n")
	}
	return sb.String()
}
