package review_service

import (
	"context"
	"fmt"
	"strings"

	"github.com/yi-nology/code-review-service/biz/handler/common"
	modelWebhook "github.com/yi-nology/code-review-service/biz/model/webhook"
	"github.com/yi-nology/code-review-service/internal/repository"
	"github.com/yi-nology/code-review-service/internal/service/review"
	"github.com/yi-nology/code-review-service/internal/service/webhook"

	"github.com/oekit/pkg/convert"
	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/hlog"
)

type WebhookHandler struct {
	webhookSvc   *webhook.Service
	reviewSvc    ReviewCreator
	platformRepo repository.GitPlatformRepository
}

type ReviewCreator interface {
	CreateFromWebhook(ctx context.Context, req review.Request, idempotencyKey string) (review.Response, error)
}

// InitWebhookHandler creates and initializes the WebhookHandler with the given dependencies.
func InitWebhookHandler(webhookSvc *webhook.Service, reviewSvc ReviewCreator, platformRepo repository.GitPlatformRepository) {
	webhookInst = &WebhookHandler{
		webhookSvc:   webhookSvc,
		reviewSvc:    reviewSvc,
		platformRepo: platformRepo,
	}
}

func (h *WebhookHandler) HandleWebhook(ctx context.Context, c *app.RequestContext) {
	var req modelWebhook.HandleWebhookReq
	if err := c.BindAndValidate(&req); err != nil {
		common.Fail(c, common.BadRequest(err.Error()))
		return
	}

	if req.Provider == "" || req.Installation == "" {
		common.Fail(c, common.BadRequest("provider and installation are required"))
		return
	}

	rawBody := req.Body
	if len(rawBody) == 0 {
		common.Fail(c, common.BadRequest("empty body"))
		return
	}

	if h.webhookSvc.MaxPayloadBytes() > 0 && int64(len(rawBody)) > h.webhookSvc.MaxPayloadBytes() {
		common.Fail(c, common.BizErr(413, "payload too large"))
		return
	}

	platform, err := h.platformRepo.GetByProviderAndInstallation(ctx, req.Provider, req.Installation)
	if err != nil {
		hlog.Error("lookup installation failed", "provider", req.Provider, "installation", req.Installation, "error", err)
		common.Fail(c, common.BizErr(500, "internal error"))
		return
	}
	if platform == nil {
		common.Fail(c, common.NotFound("installation not found"))
		return
	}

	normalizer := getNormalizer(req.Provider)
	if normalizer == nil {
		common.Fail(c, common.BadRequest(fmt.Sprintf("unsupported provider %q", req.Provider)))
		return
	}

	eventType := normalizer.EventType(string(c.GetHeader(normalizerEventTypeHeader(req.Provider))))
	deliveryID := normalizer.DeliveryID(string(c.GetHeader(normalizerDeliveryIDHeader(req.Provider))))
	signature := string(c.GetHeader(normalizerSignatureHeader(req.Provider)))

	result, err := h.webhookSvc.HandleWebhook(req.Provider, req.Installation, rawBody, eventType, deliveryID, signature, platform.WebhookSecret)
	if err != nil {
		if strings.Contains(err.Error(), "signature verification failed") {
			common.Fail(c, common.BizErr(401, "signature verification failed"))
			return
		}
		hlog.Error("webhook handling failed", "provider", req.Provider, "error", err)
		common.Fail(c, common.BizErr(500, "internal error"))
		return
	}

	if result.Duplicate {
		c.SetStatusCode(202)
		c.JSON(202, &modelWebhook.HandleWebhookResp{Status: "duplicate"})
		return
	}

	if result.Ignored {
		c.SetStatusCode(204)
		return
	}

	event := result.ChangeEvent
	idempotencyKey := fmt.Sprintf("webhook:%s:%s:%s:%d:%s",
		event.Provider,
		event.InstallationID,
		event.RepositoryID,
		event.Number,
		event.HeadSHA,
	)

	reviewReq := review.Request{
		Provider:        event.Provider,
		Repo:            event.Repository,
		ProjectID:       event.RepositoryID,
		PullRequestID:   convert.IntToStr(event.Number),
		MergeRequestIID: convert.IntToStr(event.Number),
		BaseSHA:         event.BaseSHA,
		HeadSHA:         event.HeadSHA,
		CallbackMode:    "comment",
		Author:          event.Author,
		AuthorName:      event.AuthorName,
		AuthorAvatar:    event.AuthorAvatar,
		Sender:          event.Sender,
	}

	resp, err := h.reviewSvc.CreateFromWebhook(ctx, reviewReq, idempotencyKey)
	if err != nil {
		hlog.Error("create review from webhook failed", "provider", req.Provider, "error", err)
		_ = h.webhookSvc.MarkDeliveryProcessed(ctx, req.Provider, req.Installation, deliveryID, "error", err.Error())
		common.Fail(c, common.BizErr(500, "failed to create review"))
		return
	}

	_ = h.webhookSvc.RecordRevision(ctx, event, resp.ReviewID)
	_ = h.webhookSvc.MarkDeliveryProcessed(ctx, req.Provider, req.Installation, deliveryID, "accepted", "")

	c.SetStatusCode(202)
	c.JSON(202, &modelWebhook.HandleWebhookResp{
		Status:   "accepted",
		ReviewID: &resp.ReviewID,
	})
}

func getNormalizer(provider string) webhook.Normalizer {
	switch strings.ToLower(provider) {
	case "github":
		return &webhook.GitHubNormalizer{}
	case "gitlab":
		return &webhook.GitLabNormalizer{}
	case "gitea":
		return &webhook.GiteaNormalizer{}
	case "gitcode":
		return &webhook.GitCodeNormalizer{}
	case "tencent_code":
		return &webhook.TencentCodeNormalizer{}
	}
	return nil
}

func normalizerEventTypeHeader(provider string) string {
	switch strings.ToLower(provider) {
	case "github":
		return "X-GitHub-Event"
	case "gitlab", "gitcode", "tencent_code":
		return "X-GitLab-Event"
	case "gitea":
		return "X-Gitea-Event"
	}
	return "X-Event-Type"
}

func normalizerDeliveryIDHeader(provider string) string {
	switch strings.ToLower(provider) {
	case "github":
		return "X-GitHub-Delivery"
	case "gitlab", "gitcode", "tencent_code":
		return "X-Gitlab-Event-UUID"
	case "gitea":
		return "X-Gitea-Delivery"
	}
	return "X-Delivery-ID"
}

func normalizerSignatureHeader(provider string) string {
	switch strings.ToLower(provider) {
	case "github":
		return "X-Hub-Signature-256"
	case "gitlab", "gitcode", "tencent_code":
		return "X-Gitlab-Token"
	case "gitea":
		return "X-Gitea-Signature"
	}
	return "X-Signature"
}
