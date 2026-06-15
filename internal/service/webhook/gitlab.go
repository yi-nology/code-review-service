package webhook

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type GitLabNormalizer struct{}

func (n *GitLabNormalizer) Normalize(rawBody []byte, eventType string) (*ChangeEvent, error) {
	if eventType == "Push Hook" {
		return n.normalizePush(rawBody)
	}
	if eventType == "Merge Request Hook" {
		return n.normalizeMergeRequest(rawBody)
	}
	return nil, nil
}

func (n *GitLabNormalizer) normalizePush(rawBody []byte) (*ChangeEvent, error) {
	var payload map[string]any
	if err := json.Unmarshal(rawBody, &payload); err != nil {
		return nil, fmt.Errorf("parse gitlab push payload: %w", err)
	}

	ref, _ := payload["ref"].(string)
	if ref == "" {
		return nil, nil
	}
	branch := strings.TrimPrefix(ref, "refs/heads/")
	if branch == ref {
		return nil, nil
	}

	var repoPath, repoID string
	if proj, ok := payload["project"].(map[string]any); ok {
		repoPath, _ = proj["path_with_namespace"].(string)
		repoID = fmt.Sprintf("%v", proj["id"])
	}

	headSHA, _ := payload["after"].(string)

	var sender string
	if user, ok := payload["user_username"].(string); ok {
		sender = user
	} else if user, ok := payload["user"].(map[string]any); ok {
		sender, _ = user["username"].(string)
	}

	return &ChangeEvent{
		Provider:     "gitlab",
		EventType:    "push",
		Action:       "push",
		RepositoryID: repoID,
		Repository:   repoPath,
		HeadSHA:      headSHA,
		Sender:       sender,
	}, nil
}

func (n *GitLabNormalizer) normalizeMergeRequest(rawBody []byte) (*ChangeEvent, error) {
	var payload map[string]any
	if err := json.Unmarshal(rawBody, &payload); err != nil {
		return nil, fmt.Errorf("parse gitlab payload: %w", err)
	}

	attrs, ok := payload["object_attributes"].(map[string]any)
	if !ok {
		return nil, nil
	}

	action, _ := attrs["action"].(string)
	if !IsSupportedAction("gitlab", action) {
		return nil, nil
	}

	if action == "update" {
		oldrev, _ := attrs["oldrev"].(string)
		newrev, _ := attrs["newrev"].(string)
		if oldrev == newrev || newrev == "" {
			return nil, nil
		}
	}

	iid := jsonInt(attrs["iid"])
	if iid == 0 {
		return nil, fmt.Errorf("missing merge request iid")
	}

	var headSHA, baseSHA, startSHA string
	if lastCommit, ok := attrs["last_commit"].(map[string]any); ok {
		headSHA, _ = lastCommit["id"].(string)
	}
	if source, ok := attrs["source"].(map[string]any); ok {
		if branch, ok := source["branch"].(string); ok {
			_ = branch
		}
	}
	baseSHA, _ = attrs["merge_commit_sha"].(string)
	if baseSHA == "" {
		if diffRefs, ok := attrs["diff_refs"].(map[string]any); ok {
			baseSHA, _ = diffRefs["base_sha"].(string)
			headSHA, _ = diffRefs["head_sha"].(string)
			startSHA, _ = diffRefs["start_sha"].(string)
		}
	}

	var repoPath string
	if target, ok := payload["project"].(map[string]any); ok {
		repoPath, _ = target["path_with_namespace"].(string)
	}

	var repoID string
	if proj, ok := payload["project"].(map[string]any); ok {
		repoID = fmt.Sprintf("%v", proj["id"])
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

	author := sender

	return &ChangeEvent{
		Provider:       "gitlab",
		EventType:      "merge_request",
		Action:         action,
		RepositoryID:   repoID,
		Repository:     repoPath,
		Number:         iid,
		BaseSHA:        baseSHA,
		StartSHA:       startSHA,
		HeadSHA:        headSHA,
		Sender:         sender,
		Author:         author,
		AuthorName:     authorName,
		AuthorAvatar:   authorAvatar,
	}, nil
}

func (n *GitLabNormalizer) VerifySignature(body []byte, signature string, secret string) bool {
	if secret == "" {
		return false
	}
	return strings.TrimSpace(signature) == strings.TrimSpace(secret)
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
