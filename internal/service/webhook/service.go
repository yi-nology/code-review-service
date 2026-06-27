package webhook

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/yi-nology/code-review-service/internal/repository"
	"github.com/yi-nology/code-review-service/internal/repository/postgres/models"

	"gorm.io/datatypes"
)

type ServiceConfig struct {
	Normalizers        map[string]Normalizer
	DeliveryRepo       repository.WebhookDeliveryRepository
	ChangeRevisionRepo repository.ChangeRevisionRepository
	MaxPayloadBytes    int64
	BotUsername        string
}

type Service struct {
	normalizers        map[string]Normalizer
	deliveryRepo       repository.WebhookDeliveryRepository
	changeRevisionRepo repository.ChangeRevisionRepository
	maxPayloadBytes    int64
	botUsername        string
}

func NewService(cfg ServiceConfig) *Service {
	if cfg.MaxPayloadBytes <= 0 {
		cfg.MaxPayloadBytes = 1 << 20
	}
	return &Service{
		normalizers:        cfg.Normalizers,
		deliveryRepo:       cfg.DeliveryRepo,
		changeRevisionRepo: cfg.ChangeRevisionRepo,
		maxPayloadBytes:    cfg.MaxPayloadBytes,
		botUsername:        cfg.BotUsername,
	}
}

type HandleResult struct {
	ChangeEvent *ChangeEvent
	Duplicate   bool
	Ignored     bool
	ReviewID    string
}

func (s *Service) MaxPayloadBytes() int64 {
	return s.maxPayloadBytes
}

func (s *Service) GetNormalizer(provider string) (Normalizer, bool) {
	n, ok := s.normalizers[provider]
	return n, ok
}

func (s *Service) HandleWebhook(ctx context.Context, provider, installationID string, rawBody []byte, eventType, deliveryID, signature string, secret string) (*HandleResult, error) {
	normalizer, ok := s.normalizers[provider]
	if !ok {
		return nil, fmt.Errorf("unsupported provider %q", provider)
	}

	if !normalizer.VerifySignature(rawBody, signature, secret) {
		return nil, fmt.Errorf("signature verification failed")
	}

	h := sha256.Sum256(rawBody)
	payloadSHA256 := hex.EncodeToString(h[:])

	existing, err := s.deliveryRepo.Get(ctx, provider, installationID, deliveryID)
	if err != nil {
		return nil, fmt.Errorf("check delivery: %w", err)
	}
	if existing != nil {
		return &HandleResult{Duplicate: true}, nil
	}

	delivery := &models.WebhookDelivery{
		Provider:       provider,
		InstallationID: installationID,
		DeliveryID:     deliveryID,
		EventType:      eventType,
		Action:         "received",
		PayloadSHA256:  payloadSHA256,
		Payload:        datatypes.JSON(rawBody),
		Status:         "received",
		ReceivedAt:     time.Now().UTC(),
	}
	if err := s.deliveryRepo.Create(ctx, delivery); err != nil {
		return nil, fmt.Errorf("persist delivery: %w", err)
	}

	var payload map[string]any
	_ = json.Unmarshal(rawBody, &payload)

	if IsBotSender(provider, payload, s.botUsername) {
		_ = s.deliveryRepo.MarkProcessed(ctx, provider, installationID, deliveryID, "ignored", "bot sender")
		return &HandleResult{Ignored: true}, nil
	}

	event, err := normalizer.Normalize(rawBody, eventType)
	if err != nil {
		return nil, fmt.Errorf("normalize: %w", err)
	}
	if event == nil {
		_ = s.deliveryRepo.MarkProcessed(ctx, provider, installationID, deliveryID, "ignored", "")
		return &HandleResult{Ignored: true}, nil
	}

	if IsDraftEvent(provider, payload) {
		_ = s.deliveryRepo.MarkProcessed(ctx, provider, installationID, deliveryID, "ignored", "draft PR")
		return &HandleResult{Ignored: true}, nil
	}

	event.InstallationID = installationID

	return &HandleResult{ChangeEvent: event}, nil
}

func (s *Service) RecordRevision(ctx context.Context, event *ChangeEvent, reviewID string) error {
	return s.changeRevisionRepo.Upsert(ctx, &models.ChangeRevision{
		Provider:       event.Provider,
		InstallationID: event.InstallationID,
		RepositoryID:   event.RepositoryID,
		Repository:     event.Repository,
		ChangeNumber:   event.Number,
		BaseSHA:        event.BaseSHA,
		StartSHA:       event.StartSHA,
		HeadSHA:        event.HeadSHA,
		LatestReviewID: reviewID,
		CreatedAt:      time.Now().UTC(),
	})
}

func (s *Service) MarkDeliveryProcessed(ctx context.Context, provider, installationID, deliveryID, status, errMsg string) error {
	return s.deliveryRepo.MarkProcessed(ctx, provider, installationID, deliveryID, status, errMsg)
}
