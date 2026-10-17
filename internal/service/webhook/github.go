package webhook

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

type GitHubNormalizer struct {
	providers ProviderGetter
}

// NewGitHubNormalizer 构造一个委托 SDK 解析 payload 的 GitHub normalizer。
func NewGitHubNormalizer(providers ProviderGetter) *GitHubNormalizer {
	return &GitHubNormalizer{providers: providers}
}

// Normalize 委托 go-git-platform 的 WebhookManager.ParseWebhookEvent 解析
// GitHub webhook payload，再映射为项目 ChangeEvent。
func (n *GitHubNormalizer) Normalize(rawBody []byte, eventType string) (*ChangeEvent, error) {
	return normalizeWithSDK(n.providers, "github", rawBody, eventType)
}

func (n *GitHubNormalizer) VerifySignature(body []byte, signature string, secret string) bool {
	if secret == "" {
		return false
	}
	if !strings.HasPrefix(signature, "sha256=") {
		return false
	}
	sig, err := hex.DecodeString(strings.TrimPrefix(signature, "sha256="))
	if err != nil {
		return false
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	expected := mac.Sum(nil)
	return hmac.Equal(sig, expected)
}

func (n *GitHubNormalizer) EventType(headerValue string) string {
	return headerValue
}

func (n *GitHubNormalizer) DeliveryID(headerValue string) string {
	return headerValue
}
