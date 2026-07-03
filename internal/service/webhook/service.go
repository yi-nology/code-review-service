package webhook

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/yi-nology/code-review-service/internal/domain/revision"
	domainWebhook "github.com/yi-nology/code-review-service/internal/domain/webhook"
	"github.com/yi-nology/code-review-service/internal/repository"
	"github.com/yi-nology/code-review-service/internal/repository/repoerr"

	"github.com/oekit/logx"
)

// ErrSignatureMismatch 表示 webhook 签名校验失败。
// 作为哨兵错误供 handler 用 errors.Is 精确判定（替代脆弱的字符串匹配）。
var ErrSignatureMismatch = errors.New("signature verification failed")

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

func NewService(cfg ServiceConfig) (*Service, error) {
	if cfg.MaxPayloadBytes <= 0 {
		cfg.MaxPayloadBytes = 1 << 20
	}
	// PR-07 修复：botUsername 为空时启动阶段直接报错，
	// 避免运行时静默放过 bot 自身的回声事件。
	if cfg.BotUsername == "" {
		return nil, fmt.Errorf("webhook service: BotUsername is required to ignore bot-sent webhooks; configure it to your bot's login/username (e.g. \"review-bot\")")
	}
	return &Service{
		normalizers:        cfg.Normalizers,
		deliveryRepo:       cfg.DeliveryRepo,
		changeRevisionRepo: cfg.ChangeRevisionRepo,
		maxPayloadBytes:    cfg.MaxPayloadBytes,
		botUsername:        cfg.BotUsername,
	}, nil
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
		return nil, ErrSignatureMismatch
	}

	h := sha256.Sum256(rawBody)
	payloadSHA256 := hex.EncodeToString(h[:])

	existing, err := s.deliveryRepo.Get(ctx, provider, installationID, deliveryID)
	if err != nil && !errors.Is(err, repoerr.ErrNotFound) {
		return nil, fmt.Errorf("check delivery: %w", err)
	}
	if existing != nil {
		return &HandleResult{Duplicate: true}, nil
	}

	delivery := &domainWebhook.Delivery{
		Provider:       provider,
		InstallationID: installationID,
		DeliveryID:     deliveryID,
		EventType:      eventType,
		Action:         "received",
		PayloadSHA256:  payloadSHA256,
		Payload:        json.RawMessage(rawBody),
		Status:         "received",
		ReceivedAt:     time.Now().UTC(),
	}
	if err := s.deliveryRepo.Create(ctx, delivery); err != nil {
		return nil, fmt.Errorf("persist delivery: %w", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(rawBody, &payload); err != nil {
		// 签名已验证，body 必为平台原始报文。解析失败不阻断（Normalize 会用自己的解析报错），
		// 但此前被静默吞掉，畸形 payload 会被误判为「非 bot」继续流转。此处记录告警便于诊断。
		logx.Warnf("webhook payload is not valid JSON for provider=%s delivery=%s: %v", provider, deliveryID, err)
	}

	if IsBotSender(provider, payload, s.botUsername) {
		// PR-07 修复：MarkProcessed 错误不再 silent warn，回传给调用方。
		if err := s.deliveryRepo.MarkProcessed(ctx, provider, installationID, deliveryID, "ignored", "bot sender"); err != nil {
			return &HandleResult{Ignored: true}, fmt.Errorf("mark delivery as ignored (bot sender): %w", err)
		}
		return &HandleResult{Ignored: true}, nil
	}

	event, err := normalizer.Normalize(rawBody, eventType)
	if err != nil {
		return nil, fmt.Errorf("normalize: %w", err)
	}
	if event == nil {
		if err := s.deliveryRepo.MarkProcessed(ctx, provider, installationID, deliveryID, "ignored", ""); err != nil {
			return &HandleResult{Ignored: true}, fmt.Errorf("mark delivery as ignored (nil event): %w", err)
		}
		return &HandleResult{Ignored: true}, nil
	}

	if IsDraftEvent(provider, payload) {
		if err := s.deliveryRepo.MarkProcessed(ctx, provider, installationID, deliveryID, "ignored", "draft PR"); err != nil {
			return &HandleResult{Ignored: true}, fmt.Errorf("mark delivery as ignored (draft PR): %w", err)
		}
		return &HandleResult{Ignored: true}, nil
	}

	event.InstallationID = installationID

	return &HandleResult{ChangeEvent: event}, nil
}

func (s *Service) RecordRevision(ctx context.Context, event *ChangeEvent, reviewID string) error {
	return s.changeRevisionRepo.Upsert(ctx, &revision.Change{
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
