package webhook

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
)

type GiteaNormalizer struct{}

func (n *GiteaNormalizer) Normalize(rawBody []byte, eventType string) (*ChangeEvent, error) {
	// Handle push events - trigger review for PRs targeting the pushed branch
	if eventType == "push" {
		return n.normalizePush(rawBody)
	}

	// Handle pull_request events
	if eventType == "pull_request" {
		return n.normalizePullRequest(rawBody)
	}

	return nil, nil
}

func (n *GiteaNormalizer) normalizePush(rawBody []byte) (*ChangeEvent, error) {
	var payload map[string]any
	if err := json.Unmarshal(rawBody, &payload); err != nil {
		return nil, fmt.Errorf("parse gitea push payload: %w", err)
	}

	// Extract branch name from ref
	ref, _ := payload["ref"].(string)
	if ref == "" {
		return nil, nil
	}
	// Extract branch name from refs/heads/xxx
	branch := strings.TrimPrefix(ref, "refs/heads/")
	if branch == ref {
		return nil, nil
	}

	// Extract repository info
	var repoFullName, repoID string
	if repo, ok := payload["repository"].(map[string]any); ok {
		repoFullName, _ = repo["full_name"].(string)
		repoID = fmt.Sprintf("%v", repo["id"])
	}

	// Extract head SHA
	headSHA, _ := payload["after"].(string)

	// Extract sender
	var sender string
	if s, ok := payload["sender"].(map[string]any); ok {
		sender, _ = s["login"].(string)
	}

	// For push events, we don't have a specific PR number
	// The webhook handler will need to find open PRs for this branch
	return &ChangeEvent{
		Provider:     "gitea",
		EventType:    "push",
		Action:       "push",
		RepositoryID: repoID,
		Repository:   repoFullName,
		HeadSHA:      headSHA,
		Sender:       sender,
		// Number will be 0 for push events - handler needs to find the PR
	}, nil
}

func (n *GiteaNormalizer) normalizePullRequest(rawBody []byte) (*ChangeEvent, error) {
	var payload map[string]any
	if err := json.Unmarshal(rawBody, &payload); err != nil {
		return nil, fmt.Errorf("parse gitea payload: %w", err)
	}

	action, _ := payload["action"].(string)
	if !IsSupportedAction("gitea", action) {
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
		if headSHA == "" {
			headSHA, _ = head["label"].(string)
		}
	}
	if base, ok := pr["base"].(map[string]any); ok {
		baseSHA, _ = base["sha"].(string)
		if baseSHA == "" {
			baseSHA, _ = base["label"].(string)
		}
	}

	var repoFullName, repoID string
	if repo, ok := payload["repository"].(map[string]any); ok {
		repoFullName, _ = repo["full_name"].(string)
		repoID = fmt.Sprintf("%v", repo["id"])
	}

	var sender string
	if s, ok := payload["sender"].(map[string]any); ok {
		sender, _ = s["login"].(string)
	}

	var author, authorName, authorAvatar string
	if user, ok := pr["user"].(map[string]any); ok {
		author, _ = user["login"].(string)
		authorName, _ = user["full_name"].(string)
		if authorName == "" {
			authorName = author
		}
		authorAvatar, _ = user["avatar_url"].(string)
	}

	deliveryID := fmt.Sprintf("gitea-%v", payload["delivery_id"])

	return &ChangeEvent{
		Provider:     "gitea",
		DeliveryID:   deliveryID,
		EventType:    "pull_request",
		Action:       action,
		RepositoryID: repoID,
		Repository:   repoFullName,
		Number:       number,
		BaseSHA:      baseSHA,
		HeadSHA:      headSHA,
		Sender:       sender,
		Author:       author,
		AuthorName:   authorName,
		AuthorAvatar: authorAvatar,
	}, nil
}

func (n *GiteaNormalizer) VerifySignature(body []byte, signature string, secret string) bool {
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

func (n *GiteaNormalizer) EventType(headerValue string) string {
	return headerValue
}

func (n *GiteaNormalizer) DeliveryID(headerValue string) string {
	if headerValue != "" {
		return headerValue
	}
	return "gitea-unknown"
}
