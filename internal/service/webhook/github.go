package webhook

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
)

type GitHubNormalizer struct{}

func (n *GitHubNormalizer) Normalize(rawBody []byte, eventType string) (*ChangeEvent, error) {
	if eventType == "push" {
		return n.normalizePush(rawBody)
	}
	if eventType == "pull_request" {
		return n.normalizePullRequest(rawBody)
	}
	return nil, nil
}

func (n *GitHubNormalizer) normalizePush(rawBody []byte) (*ChangeEvent, error) {
	var payload map[string]any
	if err := json.Unmarshal(rawBody, &payload); err != nil {
		return nil, fmt.Errorf("parse github push payload: %w", err)
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
		repoID = fmt.Sprintf("%v", repo["id"])
	}

	headSHA, _ := payload["after"].(string)

	var sender string
	if s, ok := payload["sender"].(map[string]any); ok {
		sender, _ = s["login"].(string)
	}

	return &ChangeEvent{
		Provider:     "github",
		EventType:    "push",
		Action:       "push",
		RepositoryID: repoID,
		Repository:   repoFullName,
		HeadSHA:      headSHA,
		Sender:       sender,
	}, nil
}

func (n *GitHubNormalizer) normalizePullRequest(rawBody []byte) (*ChangeEvent, error) {
	var payload map[string]any
	if err := json.Unmarshal(rawBody, &payload); err != nil {
		return nil, fmt.Errorf("parse github payload: %w", err)
	}

	action, _ := payload["action"].(string)
	if !IsSupportedAction("github", action) {
		return nil, nil
	}

	pr, ok := payload["pull_request"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("missing pull_request in payload")
	}

	number := jsonInt(pr["number"])
	if number == 0 {
		return nil, fmt.Errorf("missing pull request number")
	}

	var headSHA, baseSHA string
	if head, ok := pr["head"].(map[string]any); ok {
		headSHA, _ = head["sha"].(string)
	}
	if base, ok := pr["base"].(map[string]any); ok {
		baseSHA, _ = base["sha"].(string)
	}

	var repoFullName, repoID string
	if repo, ok := payload["repository"].(map[string]any); ok {
		repoFullName, _ = repo["full_name"].(string)
		repoID = fmt.Sprintf("%v", repo["id"])
	}

	deliveryID := fmt.Sprintf("%v", payload["delivery_id"])

	var sender string
	if s, ok := payload["sender"].(map[string]any); ok {
		sender, _ = s["login"].(string)
	}

	var author, authorName, authorAvatar string
	if user, ok := pr["user"].(map[string]any); ok {
		author, _ = user["login"].(string)
		authorName, _ = user["login"].(string)
		authorAvatar, _ = user["avatar_url"].(string)
	}

	return &ChangeEvent{
		Provider:       "github",
		DeliveryID:     deliveryID,
		EventType:      "pull_request",
		Action:         action,
		RepositoryID:   repoID,
		Repository:     repoFullName,
		Number:         number,
		BaseSHA:        baseSHA,
		HeadSHA:        headSHA,
		Sender:         sender,
		Author:         author,
		AuthorName:     authorName,
		AuthorAvatar:   authorAvatar,
	}, nil
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
