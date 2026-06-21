package webhook

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/oekit/pkg/convert"
)

type TencentCodeNormalizer struct{}

func (n *TencentCodeNormalizer) Normalize(rawBody []byte, eventType string) (*ChangeEvent, error) {
	if eventType == "Push Hook" {
		return n.normalizePush(rawBody)
	}
	if eventType == "Merge Request Hook" {
		return n.normalizeMergeRequest(rawBody)
	}
	return nil, nil
}

func (n *TencentCodeNormalizer) normalizePush(rawBody []byte) (*ChangeEvent, error) {
	var payload map[string]any
	if err := json.Unmarshal(rawBody, &payload); err != nil {
		return nil, fmt.Errorf("parse tencent push payload: %w", err)
	}

	ref, _ := payload["ref"].(string)
	if ref == "" {
		return nil, nil
	}
	branch := strings.TrimPrefix(ref, "refs/heads/")
	if branch == ref {
		return nil, nil
	}

	var repoFullName, repoID string
	if repo, ok := payload["repository"].(map[string]any); ok {
		repoFullName, _ = repo["full_name"].(string)
		repoID = convert.AnyToStr(repo["id"])
	}

	headSHA, _ := payload["after"].(string)

	var sender string
	if user, ok := payload["user_username"].(string); ok {
		sender = user
	} else if user, ok := payload["user"].(map[string]any); ok {
		sender, _ = user["username"].(string)
	}

	return &ChangeEvent{
		Provider:     "tencent_code",
		EventType:    "push",
		Action:       "push",
		RepositoryID: repoID,
		Repository:   repoFullName,
		HeadSHA:      headSHA,
		Sender:       sender,
	}, nil
}

func (n *TencentCodeNormalizer) normalizeMergeRequest(rawBody []byte) (*ChangeEvent, error) {
	var payload map[string]any
	if err := json.Unmarshal(rawBody, &payload); err != nil {
		return nil, fmt.Errorf("parse tencent payload: %w", err)
	}

	attrs, ok := payload["object_attributes"].(map[string]any)
	if !ok {
		return nil, nil
	}

	action, _ := attrs["action"].(string)
	if !IsSupportedAction("tencent_code", action) {
		return nil, nil
	}

	iid := jsonInt(attrs["iid"])
	if iid == 0 {
		return nil, fmt.Errorf("missing merge request iid")
	}

	var headSHA, baseSHA string
	if lastCommit, ok := attrs["last_commit"].(map[string]any); ok {
		headSHA, _ = lastCommit["id"].(string)
	}
	baseSHA, _ = attrs["merge_commit_sha"].(string)
	if baseSHA == "" {
		if diffRefs, ok := attrs["diff_refs"].(map[string]any); ok {
			baseSHA, _ = diffRefs["base_sha"].(string)
			headSHA, _ = diffRefs["head_sha"].(string)
		}
	}

	var repoPath, repoID string
	if proj, ok := payload["project"].(map[string]any); ok {
		repoPath, _ = proj["path_with_namespace"].(string)
		repoID = convert.AnyToStr(proj["id"])
	}

	var sender string
	var authorName, authorAvatar string
	if user, ok := payload["user"].(map[string]any); ok {
		sender, _ = user["username"].(string)
		authorName, _ = user["name"].(string)
		if authorName == "" {
			authorName = sender
		}
		authorAvatar, _ = user["avatar_url"].(string)
	}

	return &ChangeEvent{
		Provider:     "tencent_code",
		EventType:    "merge_request",
		Action:       action,
		RepositoryID: repoID,
		Repository:   repoPath,
		Number:       iid,
		BaseSHA:      baseSHA,
		HeadSHA:      headSHA,
		Sender:       sender,
		Author:       sender,
		AuthorName:   authorName,
		AuthorAvatar: authorAvatar,
	}, nil
}

func (n *TencentCodeNormalizer) VerifySignature(body []byte, signature string, secret string) bool {
	if secret == "" {
		return true
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
	return "tencent-unknown"
}
