package webhook

import (
	"crypto/subtle"
	"fmt"
	"strings"
	"time"
)

type GitLabNormalizer struct {
	providers ProviderGetter
}

// NewGitLabNormalizer 构造一个委托 SDK 解析 payload 的 GitLab normalizer。
func NewGitLabNormalizer(providers ProviderGetter) *GitLabNormalizer {
	return &GitLabNormalizer{providers: providers}
}

// Normalize 委托 git-platform-sdk 的 WebhookManager.ParseWebhookEvent 解析
// GitLab webhook payload，再映射为项目 ChangeEvent。
func (n *GitLabNormalizer) Normalize(rawBody []byte, eventType string) (*ChangeEvent, error) {
	return normalizeWithSDK(n.providers, "gitlab", rawBody, eventType)
}

func (n *GitLabNormalizer) VerifySignature(body []byte, signature string, secret string) bool {
	if secret == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(strings.TrimSpace(signature)), []byte(strings.TrimSpace(secret))) == 1
}

func (n *GitLabNormalizer) EventType(headerValue string) string {
	return headerValue
}

func (n *GitLabNormalizer) DeliveryID(headerValue string) string {
	if headerValue != "" {
		return headerValue
	}
	return fmt.Sprintf("gl-%d", time.Now().UnixNano())
}
