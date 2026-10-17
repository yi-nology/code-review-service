package webhook

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

type GiteaNormalizer struct {
	providers ProviderGetter
}

// NewGiteaNormalizer 构造一个委托 SDK 解析 payload 的 Gitea normalizer。
func NewGiteaNormalizer(providers ProviderGetter) *GiteaNormalizer {
	return &GiteaNormalizer{providers: providers}
}

// Normalize 委托 go-git-platform 的 WebhookManager.ParseWebhookEvent 解析
// Gitea webhook payload，再映射为项目 ChangeEvent。
func (n *GiteaNormalizer) Normalize(rawBody []byte, eventType string) (*ChangeEvent, error) {
	return normalizeWithSDK(n.providers, "gitea", rawBody, eventType)
}

func (n *GiteaNormalizer) VerifySignature(body []byte, signature string, secret string) bool {
	if secret == "" {
		return allowInsecureSignature()
	}
	if signature == "" {
		return false
	}

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	expectedMAC := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(strings.TrimSpace(signature)), []byte(expectedMAC))
}

func (n *GiteaNormalizer) EventType(headerValue string) string {
	return headerValue
}

func (n *GiteaNormalizer) DeliveryID(headerValue string) string {
	if headerValue != "" {
		return headerValue
	}
	return fmt.Sprintf("gitea-%d", time.Now().UnixNano())
}
