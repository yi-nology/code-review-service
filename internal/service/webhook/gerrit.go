package webhook

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// GerritNormalizer 解析 Gerrit webhooks 插件推送的事件。
//
// Gerrit 没有内置 webhook，需要安装 webhooks 插件。
// 插件将 Gerrit stream-events 格式的 JSON 通过 HTTP POST 推送到配置的 URL。
//
// 支持的事件类型：
//   - patchset-created: 新 patchset 上传时触发（主触发条件）
//   - comment-added: 评论添加时触发（可选，暂不处理）
//
// Payload 格式（基于 Gerrit stream-events）：
//
//	{
//	  "type": "patchset-created",
//	  "change": {
//	    "project": "my-project",
//	    "branch": "main",
//	    "id": "I1234abcd",
//	    "number": 12345,
//	    "subject": "Fix bug",
//	    "status": "NEW",
//	    "owner": {"name": "User", "username": "user", "email": "user@example.com"}
//	  },
//	  "patchSet": {
//	    "number": 3,
//	    "revision": "abc123def456",
//	    "ref": "refs/changes/45/12345/3",
//	    "uploader": {"name": "User", "username": "user"}
//	  }
//	}
type GerritNormalizer struct{}

// NewGerritNormalizer 构造 Gerrit webhook normalizer。
func NewGerritNormalizer() *GerritNormalizer {
	return &GerritNormalizer{}
}

// gerritWebhookEvent 是 Gerrit webhooks 插件推送的顶层事件结构。
type gerritWebhookEvent struct {
	Type      string             `json:"type"`
	Change    gerritChangeInfo   `json:"change"`
	PatchSet  gerritPatchSetInfo `json:"patchSet,omitempty"`
	Uploader  gerritAccountInfo  `json:"uploader,omitempty"`
	Author    gerritAccountInfo  `json:"author,omitempty"`
	EventTime int64              `json:"eventCreatedOn,omitempty"`
}

// gerritChangeInfo 是 Gerrit change 的 JSON 表示。
type gerritChangeInfo struct {
	Project string            `json:"project"`
	Branch  string            `json:"branch"`
	ID      string            `json:"id"`
	Number  int               `json:"number"`
	Subject string            `json:"subject"`
	Status  string            `json:"status"`
	Owner   gerritAccountInfo `json:"owner"`
}

// gerritPatchSetInfo 是 Gerrit patchset 的 JSON 表示。
type gerritPatchSetInfo struct {
	Number   int               `json:"number"`
	Revision string            `json:"revision"`
	Ref      string            `json:"ref"`
	Uploader gerritAccountInfo `json:"uploader"`
}

// gerritAccountInfo 是 Gerrit 账户信息。
type gerritAccountInfo struct {
	Name     string `json:"name"`
	Username string `json:"username"`
	Email    string `json:"email"`
}

// supportedGerritEvents 定义会触发评审的 Gerrit 事件类型。
var supportedGerritEvents = map[string]bool{
	"patchset-created": true,
}

// Normalize 解析 Gerrit webhook payload 并映射为 ChangeEvent。
// 返回 nil 表示该事件应被忽略（不受支持的事件类型）。
func (n *GerritNormalizer) Normalize(rawBody []byte, eventType string) (*ChangeEvent, error) {
	if !supportedGerritEvents[eventType] {
		return nil, nil
	}

	var event gerritWebhookEvent
	if err := json.Unmarshal(rawBody, &event); err != nil {
		return nil, fmt.Errorf("gerrit: parse webhook payload: %w", err)
	}

	if event.Change.Number == 0 {
		return nil, fmt.Errorf("gerrit: webhook payload missing change number")
	}

	// 提取 sender/author
	sender := event.Uploader.Username
	if sender == "" {
		sender = event.Change.Owner.Username
	}
	author := event.Change.Owner.Username
	authorName := event.Change.Owner.Name
	if authorName == "" {
		authorName = author
	}

	// 提取 repository ID（Gerrit 没有数字 repo ID，用 project name 代替）
	repoID := event.Change.Project

	ce := &ChangeEvent{
		Provider:     "gerrit",
		EventType:    "patchset_created",
		Action:       "open",
		RepositoryID: repoID,
		Repository:   event.Change.Project,
		Number:       strconv.Itoa(event.Change.Number),
		HeadSHA:      event.PatchSet.Revision,
		Sender:       sender,
		Author:       author,
		AuthorName:   authorName,
	}

	return ce, nil
}

// VerifySignature 验证 Gerrit webhook 签名。
//
// Gerrit webhooks 插件的签名机制：
//   - 使用 HMAC-SHA256
//   - 签名放在 HTTP 请求头中（取决于插件配置）
//   - secret 在 Gerrit webhooks 插件配置中设定
//
// 当 secret 为空时，根据 WEBHOOK_ALLOW_INSECURE 环境变量决定是否放行。
func (n *GerritNormalizer) VerifySignature(body []byte, signature string, secret string) bool {
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

// EventType 返回 Gerrit 事件类型。
// Gerrit webhooks 插件通过 X-Gerrit-Event header 传递事件类型。
func (n *GerritNormalizer) EventType(headerValue string) string {
	return headerValue
}

// DeliveryID 返回 Gerrit 事件的投递 ID。
// Gerrit 没有原生的 delivery ID，合成一个基于时间的 fallback。
func (n *GerritNormalizer) DeliveryID(headerValue string) string {
	if headerValue != "" {
		return headerValue
	}
	return fmt.Sprintf("gerrit-%d", time.Now().UnixNano())
}
