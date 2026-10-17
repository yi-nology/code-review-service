package webhook

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

type TencentCodeNormalizer struct {
	providers ProviderGetter
}

// NewTencentCodeNormalizer 构造一个委托 SDK 解析 payload 的腾讯工蜂 normalizer。
func NewTencentCodeNormalizer(providers ProviderGetter) *TencentCodeNormalizer {
	return &TencentCodeNormalizer{providers: providers}
}

// Normalize 委托 go-git-platform 的 WebhookManager.ParseWebhookEvent 解析
// 腾讯工蜂 webhook payload，再映射为项目 ChangeEvent。
func (n *TencentCodeNormalizer) Normalize(rawBody []byte, eventType string) (*ChangeEvent, error) {
	return normalizeWithSDK(n.providers, "tencent_code", rawBody, eventType)
}

func (n *TencentCodeNormalizer) VerifySignature(body []byte, signature string, secret string) bool {
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

func (n *TencentCodeNormalizer) EventType(headerValue string) string {
	return headerValue
}

func (n *TencentCodeNormalizer) DeliveryID(headerValue string) string {
	if headerValue != "" {
		return headerValue
	}
	return fmt.Sprintf("tencent-%d", time.Now().UnixNano())
}
