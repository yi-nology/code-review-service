package webhook

import (
	"encoding/json"
	"fmt"
	"strings"
)

type GiteaNormalizer struct{}

func (n *GiteaNormalizer) Normalize(rawBody []byte, eventType string) (*ChangeEvent, error) {
	if eventType != "pull_request" {
		return nil, nil
	}

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
		Provider:       "gitea",
		DeliveryID:     deliveryID,
		EventType:      eventType,
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

func (n *GiteaNormalizer) VerifySignature(body []byte, signature string, secret string) bool {
	if secret == "" {
		return false
	}
	return strings.TrimSpace(signature) == strings.TrimSpace(secret)
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
