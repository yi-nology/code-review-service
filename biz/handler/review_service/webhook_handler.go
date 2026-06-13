package review_service

import (
	"context"
	"fmt"
	"strings"

	"github.com/yi-nology/code-review-service/internal/repository"
	"github.com/yi-nology/code-review-service/internal/service/review"
	"github.com/yi-nology/code-review-service/internal/service/webhook"

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

func NewWebhookHandler(
	webhookSvc *webhook.Service,
	reviewSvc ReviewCreator,
	platformRepo repository.GitPlatformRepository,
) *WebhookHandler {
	return &WebhookHandler{
		webhookSvc:   webhookSvc,
		reviewSvc:    reviewSvc,
		platformRepo: platformRepo,
	}
}

func (h *WebhookHandler) HandleWebhook(ctx context.Context, c *app.RequestContext) {
	provider := c.Param("provider")
	installationID := c.Param("installation")

	if provider == "" || installationID == "" {
		fail(c, badRequest("provider and installation are required"))
		return
	}

	rawBody, err := c.Body()
	if err != nil {
		fail(c, badRequest("failed to read body"))
		return
	}
	if len(rawBody) == 0 {
		fail(c, badRequest("empty body"))
		return
	}

	if h.webhookSvc.MaxPayloadBytes() > 0 && int64(len(rawBody)) > h.webhookSvc.MaxPayloadBytes() {
		fail(c, bizErr(413, "payload too large"))
		return
	}

	platform, err := h.platformRepo.GetByProviderAndInstallation(ctx, provider, installationID)
	if err != nil {
		hlog.Error("lookup installation failed", "provider", provider, "installation", installationID, "error", err)
		fail(c, bizErr(500, "internal error"))
		return
	}
	if platform == nil {
		fail(c, notFound("installation not found"))
		return
	}

	normalizer := getNormalizer(provider)
	if normalizer == nil {
		fail(c, badRequest(fmt.Sprintf("unsupported provider %q", provider)))
		return
	}

	eventType := normalizer.EventType(string(c.GetHeader(normalizerEventTypeHeader(provider))))
	deliveryID := normalizer.DeliveryID(string(c.GetHeader(normalizerDeliveryIDHeader(provider))))
	signature := string(c.GetHeader(normalizerSignatureHeader(provider)))

	result, err := h.webhookSvc.HandleWebhook(provider, installationID, rawBody, eventType, deliveryID, signature, platform.WebhookSecret)
	if err != nil {
		if strings.Contains(err.Error(), "signature verification failed") {
			fail(c, bizErr(401, "signature verification failed"))
			return
		}
		hlog.Error("webhook handling failed", "provider", provider, "error", err)
		fail(c, bizErr(500, "internal error"))
		return
	}

	if result.Duplicate {
		c.SetStatusCode(202)
		c.JSON(202, map[string]string{"status": "duplicate"})
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
		PullRequestID:   fmt.Sprintf("%d", event.Number),
		MergeRequestIID: fmt.Sprintf("%d", event.Number),
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
		hlog.Error("create review from webhook failed", "provider", provider, "error", err)
		_ = h.webhookSvc.MarkDeliveryProcessed(ctx, provider, installationID, deliveryID, "error", err.Error())
		fail(c, bizErr(500, "failed to create review"))
		return
	}

	_ = h.webhookSvc.RecordRevision(ctx, event, resp.ReviewID)
	_ = h.webhookSvc.MarkDeliveryProcessed(ctx, provider, installationID, deliveryID, "accepted", "")

	c.SetStatusCode(202)
	c.JSON(202, map[string]string{
		"status":    "accepted",
		"review_id": resp.ReviewID,
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
	}
	return nil
}

func normalizerEventTypeHeader(provider string) string {
	switch strings.ToLower(provider) {
	case "github":
		return "X-GitHub-Event"
	case "gitlab":
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
	case "gitlab":
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
	case "gitlab":
		return "X-Gitlab-Token"
	case "gitea":
		return "X-Gitea-Signature"
	}
	return "X-Signature"
}
