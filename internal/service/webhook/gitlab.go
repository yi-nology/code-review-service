package webhook

import (
	"crypto/subtle"
	"fmt"
	"regexp"
	"strings"
	"time"
)

// gitlabSpaceTimeRe 匹配 GitLab CE webhook payload 中的非标准时间格式。
// GitLab CE 部分 timestamp 字段使用 "2026-07-15 21:53:38 +0800" 格式
// (空格分隔日期/时间, 时区无冒号), 而 SDK 按 RFC3339 (T 分隔, 带冒号时区) 解析。
// 此 regex 将其修正为 "2026-07-15T21:53:38+08:00"。
var gitlabSpaceTimeRe = regexp.MustCompile(`"(\d{4}-\d{2}-\d{2}) (\d{2}:\d{2}:\d{2}) ([+-])(\d{2}):?(\d{2})"`)

// fixGitLabTimestamps 把 GitLab CE payload 中的非标准时间格式修正为 RFC3339。
func fixGitLabTimestamps(raw []byte) []byte {
	return gitlabSpaceTimeRe.ReplaceAll(raw, []byte(`"${1}T${2}${3}${4}:${5}"`))
}

type GitLabNormalizer struct {
	providers ProviderGetter
}

// NewGitLabNormalizer 构造一个委托 SDK 解析 payload 的 GitLab normalizer。
func NewGitLabNormalizer(providers ProviderGetter) *GitLabNormalizer {
	return &GitLabNormalizer{providers: providers}
}

// Normalize 委托 go-git-platform 的 WebhookManager.ParseWebhookEvent 解析
// GitLab webhook payload，再映射为项目 ChangeEvent。
func (n *GitLabNormalizer) Normalize(rawBody []byte, eventType string) (*ChangeEvent, error) {
	return normalizeWithSDK(n.providers, "gitlab", fixGitLabTimestamps(rawBody), eventType)
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
