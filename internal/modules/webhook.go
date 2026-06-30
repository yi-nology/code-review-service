package modules

import (
	"github.com/yi-nology/code-review-service/internal/config"
	"github.com/yi-nology/code-review-service/internal/repository"
	"github.com/yi-nology/code-review-service/internal/service/webhook"

	"go.uber.org/fx"
)

var WebhookModule = fx.Options(
	fx.Provide(NewWebhookService),
)

func NewWebhookService(cfg *config.Config, repos *repository.RepoContainer) (*webhook.Service, error) {
	return webhook.NewService(webhook.ServiceConfig{
		Normalizers: map[string]webhook.Normalizer{
			"github":       &webhook.GitHubNormalizer{},
			"gitlab":       &webhook.GitLabNormalizer{},
			"gitea":        &webhook.GiteaNormalizer{},
			"gitcode":      &webhook.GitCodeNormalizer{},
			"tencent_code": &webhook.TencentCodeNormalizer{},
		},
		DeliveryRepo:       repos.WebhookDelivery,
		ChangeRevisionRepo: repos.ChangeRevision,
		MaxPayloadBytes:    1 << 20,
		BotUsername:        cfg.Webhook.BotUsername,
	})
}
