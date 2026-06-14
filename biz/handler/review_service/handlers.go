package review_service

import (
	"sync"
)

var (
	reviewHandlerOnce    sync.Once
	reviewHandlerInst    *ReviewHandler

	statsHandlerOnce     sync.Once
	statsHandlerInst     *StatsHandler

	sessionHandlerOnce   sync.Once
	sessionHandlerInst   *SessionHandler

	progressWSHandlerOnce sync.Once
	progressWSHandlerInst *ProgressWSHandler

	reviewMapHandlerOnce sync.Once
	reviewMapHandlerInst *ReviewMapHandler

	publisherHandlerOnce sync.Once
	publisherHandlerInst *PublisherHandler

	projectAgentHandlerOnce sync.Once
	projectAgentHandlerInst *ProjectAgentHandler

	webhookHandlerOnce  sync.Once
	webhookHandlerInst  *WebhookHandler
)

func InitReviewHandler(h *ReviewHandler) {
	reviewHandlerOnce.Do(func() { reviewHandlerInst = h })
}

func InitStatsHandler(h *StatsHandler) {
	statsHandlerOnce.Do(func() { statsHandlerInst = h })
}

func InitSessionHandler(h *SessionHandler) {
	sessionHandlerOnce.Do(func() { sessionHandlerInst = h })
}

func InitProgressWSHandler(h *ProgressWSHandler) {
	progressWSHandlerOnce.Do(func() { progressWSHandlerInst = h })
}

func InitReviewMapHandler(h *ReviewMapHandler) {
	reviewMapHandlerOnce.Do(func() { reviewMapHandlerInst = h })
}

func InitPublisherHandler(h *PublisherHandler) {
	publisherHandlerOnce.Do(func() { publisherHandlerInst = h })
}

func InitProjectAgentHandler(h *ProjectAgentHandler) {
	projectAgentHandlerOnce.Do(func() { projectAgentHandlerInst = h })
}

func InitWebhookHandler(h *WebhookHandler) {
	webhookHandlerOnce.Do(func() { webhookHandlerInst = h })
}
