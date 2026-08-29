package webhook

// GitCode normalizer：payload 为 GitLab 风格（object_attributes / project /
// object_kind），而 SDK gitcode backend 期望 Gitea 风格（pull_request key），
// 二者不兼容。因此解析复用 SDK gitlab backend——它是纯 JSON 解析，不访问
// 平台 API，用包级懒加载的 parse-only 实例，不要求 gitlab 平台已配置。
// 签名校验（HMAC-SHA256）与 header 约定保留 GitCode 自身行为。

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	sdkprovider "github.com/yi-nology/git-platform-sdk/provider"
)

type GitCodeNormalizer struct {
	// providers 仅为构造签名统一保留；解析走包级 gitlab-family parser。
	providers ProviderGetter
}

// NewGitCodeNormalizer 构造一个 GitCode normalizer。
func NewGitCodeNormalizer(providers ProviderGetter) *GitCodeNormalizer {
	return &GitCodeNormalizer{providers: providers}
}

var (
	gitlabParseOnce     sync.Once
	gitlabParseProvider sdkprovider.Provider
	gitlabParseErr      error
)

// gitlabFamilyParser 返回仅供 webhook 解析使用的 gitlab provider 实例。
func gitlabFamilyParser() (sdkprovider.Provider, error) {
	gitlabParseOnce.Do(func() {
		gitlabParseProvider, gitlabParseErr = sdkprovider.NewProvider(sdkprovider.Config{
			Platform: sdkprovider.PlatformGitLab,
		})
	})
	return gitlabParseProvider, gitlabParseErr
}

// gitcodeHookKind 把 GitCode 的事件 header 值映射为 GitLab 协议的 object_kind。
func gitcodeHookKind(eventType string) string {
	switch eventType {
	case "Push Hook":
		return "push"
	case "Merge Request Hook":
		return "merge_request"
	}
	return ""
}

func (n *GitCodeNormalizer) Normalize(rawBody []byte, eventType string) (*ChangeEvent, error) {
	kind := gitcodeHookKind(eventType)
	if kind == "" {
		return nil, nil
	}
	body := ensureObjectKind(rawBody, kind)

	p, err := gitlabFamilyParser()
	if err != nil {
		return nil, fmt.Errorf("gitcode parse: init gitlab-family parser: %w", err)
	}
	req, _ := http.NewRequest(http.MethodPost, "/webhook", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	ne, err := p.ParseWebhookEvent(req, "")
	if err != nil {
		return nil, fmt.Errorf("gitcode parse webhook: %w", err)
	}
	if ne == nil {
		return nil, nil
	}

	ce := mapNormalizedEvent("gitcode", ne, rawBody)
	if ce == nil {
		return nil, nil
	}
	// GitCode/GitLab 的 push payload 把发送者放在顶层 user_username（无嵌套
	// user 对象），SDK gitlab backend 的 push 分支不填 Actor，此处兜底。
	if kind == "push" && ce.Sender == "" {
		ce.Sender = gitcodePushSender(rawBody)
	}
	return ce, nil
}

// gitcodePushSender 从 push payload 顶层提取发送者用户名。
func gitcodePushSender(rawBody []byte) string {
	var m map[string]any
	if json.Unmarshal(rawBody, &m) != nil {
		return ""
	}
	if s, _ := m["user_username"].(string); s != "" {
		return s
	}
	if user, ok := m["user"].(map[string]any); ok {
		s, _ := user["username"].(string)
		return s
	}
	return ""
}

// ensureObjectKind 保证 body 带 object_kind：SDK gitlab backend 依赖它判定
// 事件类型，个别 GitCode 版本的 payload 可能缺失，按 eventType header 补写。
func ensureObjectKind(rawBody []byte, kind string) []byte {
	var m map[string]json.RawMessage
	if json.Unmarshal(rawBody, &m) != nil {
		return rawBody
	}
	if _, ok := m["object_kind"]; ok {
		return rawBody
	}
	m["object_kind"] = json.RawMessage(strconv.Quote(kind))
	out, err := json.Marshal(m)
	if err != nil {
		return rawBody
	}
	return out
}

func (n *GitCodeNormalizer) VerifySignature(body []byte, signature string, secret string) bool {
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

func (n *GitCodeNormalizer) EventType(headerValue string) string {
	return headerValue
}

func (n *GitCodeNormalizer) DeliveryID(headerValue string) string {
	if headerValue != "" {
		return headerValue
	}
	return fmt.Sprintf("gitcode-%d", time.Now().UnixNano())
}
