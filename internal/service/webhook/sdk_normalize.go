// Package webhook 的 SDK 归一化适配层。
//
// 本文件用 git-platform-sdk 的 WebhookManager.ParseWebhookEvent 取代历史上
// 5 个平台各自手写的 JSON 解析逻辑（normalizePush / normalizePullRequest /
// normalizeMergeRequest）。SDK 解析后产出统一的 provider.NormalizedEvent，
// 再由 mapNormalizedEvent 映射为本项目的 ChangeEvent。
//
// 兼容性说明：
//   - github / gitea / gitlab / tencent_code 的 webhook payload 模型与 SDK
//     一致，走各自身 backend 解析。
//   - gitcode 的 payload 为 GitLab 风格（object_attributes），而 SDK gitcode
//     backend 期望 Gitea 风格（pull_request key），因此复用 SDK gitlab
//     backend 解析（见 gitcode.go），签名校验仍用 GitCode 自身约定。
//
// 保真处理（SDK 未暴露的字段，从 RawPayload 兜底提取）：
//   - RepositoryID：SDK 的 EventRepo 不含 repo ID（BuildEventRepo 仅填
//     FullName/Owner/Name），但项目用其生成幂等 key 和 domain.Request.ProjectID，
//     故从 RawPayload 提取。
//
// base/head/start SHA 无需兜底：SDK v0.48 起各 backend 自行填充，gitlab 家族
// 的优先级见 provider.ResolveMRSHAs（与历史 normalizer 一致）。
//
// 签名校验：本层不依赖 SDK 的签名校验。重建的 *http.Request 不携带签名
// header，且传入空 secret，使 SDK 各 backend 跳过其内部签名校验；签名校验
// 仍由各 normalizer 的 VerifySignature 负责（gitcode/tencent 的 header 约定
// 及空 secret 安全策略与 SDK 不同）。
package webhook

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/oekit/pkg/convert"
	sdkprovider "github.com/yi-nology/git-platform-sdk/provider"
)

// ProviderGetter 按平台名返回 SDK provider。
// 由 *gitprovider.GitProviderService 实现。
type ProviderGetter interface {
	GetProvider(platform string) (sdkprovider.Provider, error)
}

// sdkEventTypeHeader 是 SDK 各 backend 读取的事件类型 header。
// gitlab / tencent_code 无需 header（SDK 从 body 的 object_kind 判定）。
var sdkEventTypeHeader = map[string]string{
	"github": "X-GitHub-Event",
	"gitea":  "X-Gitea-Event",
}

// supportedWebhookEvents 列出会触发评审的 webhook 事件类型（项目 label 空间）。
var supportedWebhookEvents = map[string]map[string]bool{
	"github":       {"push": true, "pull_request": true},
	"gitea":        {"push": true, "pull_request": true},
	"gitlab":       {"push": true, "Push Hook": true, "Merge Request Hook": true},
	"tencent_code": {"push": true, "Push Hook": true, "Merge Request Hook": true},
}

// crEventTypeLabel 是 CR 事件在项目内部的 EventType 标签。
var crEventTypeLabel = map[string]string{
	"github":       "pull_request",
	"gitea":        "pull_request",
	"gitlab":       "merge_request",
	"gitcode":      "merge_request",
	"tencent_code": "merge_request",
}

// normalizeWithSDK 把 webhook payload 的解析委托给 SDK
// WebhookManager.ParseWebhookEvent，再把 NormalizedEvent 映射为 ChangeEvent。
func normalizeWithSDK(getter ProviderGetter, platform string, rawBody []byte, eventType string) (*ChangeEvent, error) {
	supported := supportedWebhookEvents[platform]
	if supported == nil || !supported[eventType] {
		return nil, nil
	}
	if getter == nil {
		return nil, fmt.Errorf("sdk provider not configured for %s", platform)
	}

	p, err := getter.GetProvider(platform)
	if err != nil {
		return nil, fmt.Errorf("get provider for %s: %w", platform, err)
	}

	req := buildSDKWebhookRequest(platform, eventType, rawBody)
	ne, err := p.ParseWebhookEvent(req, "")
	if err != nil {
		return nil, fmt.Errorf("sdk parse webhook: %w", err)
	}
	if ne == nil {
		return nil, nil
	}

	return mapNormalizedEvent(platform, ne, rawBody), nil
}

// buildSDKWebhookRequest 构造一个仅供 SDK 解析的最小 *http.Request：
// 携带 body 与事件类型 header，但不携带任何签名 header（配合空 secret 让
// SDK 跳过其内部签名校验）。
func buildSDKWebhookRequest(platform, eventType string, rawBody []byte) *http.Request {
	req, _ := http.NewRequest(http.MethodPost, "/webhook", bytes.NewReader(rawBody))
	req.Header.Set("Content-Type", "application/json")
	if h, ok := sdkEventTypeHeader[platform]; ok {
		req.Header.Set(h, eventType)
	}
	return req
}

// mapNormalizedEvent 将 SDK NormalizedEvent 映射为项目 ChangeEvent。
// 对 SDK 未暴露的字段（repo ID、部分平台 SHA），从 RawPayload 兜底提取。
// 返回 nil 表示该事件应被忽略（非 push/CR 事件、action 不受支持、GitLab
// 空更新等）。
func mapNormalizedEvent(platform string, ne *sdkprovider.NormalizedEvent, raw []byte) *ChangeEvent {
	isPush := ne.Type == "push"
	isCR := ne.CR != nil || strings.HasPrefix(ne.Type, "cr.")
	if !isPush && !isCR {
		return nil
	}

	ce := &ChangeEvent{
		Provider: platform,
		Action:   ne.Action,
	}

	if isPush {
		ce.EventType = "push"
	} else {
		ce.EventType = crEventTypeLabel[platform]
		if !IsSupportedAction(platform, ne.Action) {
			return nil
		}
		// 保留 GitLab "update" 去重：oldrev==newrev 或 newrev 为空时丢弃。
		if platform == "gitlab" && ne.Action == "update" && isGitLabNoOpUpdate(raw) {
			return nil
		}
	}

	if ne.Repo != nil {
		ce.Repository = ne.Repo.FullName
		// gitlab 家族 / gitea 的 backend 会填 EventRepo.ID；github 不填，
		// 由 extractRepoID 从 RawPayload 兜底。
		if ne.Repo.ID != 0 {
			ce.RepositoryID = strconv.FormatInt(ne.Repo.ID, 10)
		}
	}
	if ce.RepositoryID == "" {
		ce.RepositoryID = extractRepoID(platform, raw)
	}

	if ne.CR != nil {
		ce.Number = ne.CR.Number
		ce.BaseSHA = ne.CR.BaseSHA
		ce.HeadSHA = ne.CR.HeadSHA
		ce.StartSHA = ne.CR.StartSHA
	}
	if ne.CommitSHA != "" {
		ce.HeadSHA = ne.CommitSHA
	}

	if ne.Actor != nil {
		ce.Sender = ne.Actor.Username
		ce.Author = ne.Actor.Username
		ce.AuthorName = ne.Actor.Name
		ce.AuthorAvatar = ne.Actor.AvatarURL
	}
	if ce.AuthorName == "" {
		ce.AuthorName = ce.Author
	}

	return ce
}

// extractRepoID 从 RawPayload 提取仓库 ID。仅 github 需要：SDK github
// backend 不填 EventRepo.ID，而项目用 RepositoryID 生成幂等 key 和
// domain.Request.ProjectID。
func extractRepoID(platform string, raw []byte) string {
	if platform != "github" {
		return ""
	}
	var m map[string]any
	if json.Unmarshal(raw, &m) != nil {
		return ""
	}
	if repo, ok := m["repository"].(map[string]any); ok {
		return convert.AnyToStr(repo["id"])
	}
	return ""
}

// isGitLabNoOpUpdate 判定 GitLab update 事件是否为空更新（无新提交）。
func isGitLabNoOpUpdate(raw []byte) bool {
	var m map[string]any
	if json.Unmarshal(raw, &m) != nil {
		return false
	}
	attrs, ok := m["object_attributes"].(map[string]any)
	if !ok {
		return false
	}
	oldrev, _ := attrs["oldrev"].(string)
	newrev, _ := attrs["newrev"].(string)
	return oldrev == newrev || newrev == ""
}
