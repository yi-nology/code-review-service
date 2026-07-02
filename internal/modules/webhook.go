package modules

import (
	"github.com/yi-nology/code-review-service/internal/config"
	"github.com/yi-nology/code-review-service/internal/repository"
	gitProvider "github.com/yi-nology/code-review-service/internal/service/provider"
	"github.com/yi-nology/code-review-service/internal/service/webhook"

	"go.uber.org/fx"
)

var WebhookModule = fx.Options(
	fx.Provide(NewWebhookService),
)

func NewWebhookService(cfg *config.Config, repos *repository.RepoContainer, gps *gitProvider.GitProviderService) (*webhook.Service, error) {
	return webhook.NewService(webhook.ServiceConfig{
		Normalizers: map[string]webhook.Normalizer{
			"github":       webhook.NewGitHubNormalizer(gps),
			"gitlab":       webhook.NewGitLabNormalizer(gps),
			"gitea":        webhook.NewGiteaNormalizer(gps),
			"gitcode":      webhook.NewGitCodeNormalizer(gps),
			"tencent_code": webhook.NewTencentCodeNormalizer(gps),
		},
		DeliveryRepo:       repos.WebhookDelivery,
		ChangeRevisionRepo: repos.ChangeRevision,
		MaxPayloadBytes:    1 << 20,
		BotUsername:        cfg.Webhook.BotUsername,
	})
}
