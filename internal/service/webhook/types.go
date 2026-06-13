package webhook

import (
	"encoding/json"
	"fmt"
	"time"
)

type ChangeEvent struct {
	Provider       string
	InstallationID string
	DeliveryID     string
	EventType      string
	Action         string
	RepositoryID   string
	Repository     string
	Number         int
	BaseSHA        string
	StartSHA       string
	HeadSHA        string
	Sender         string
	Author         string
	AuthorName     string
	AuthorAvatar   string
	ReceivedAt     time.Time
}

type Normalizer interface {
	Normalize(rawBody []byte, eventType string) (*ChangeEvent, error)
	VerifySignature(body []byte, signature string, secret string) bool
	EventType(headerValue string) string
	DeliveryID(headerValue string) string
}

var supportedActions = map[string]map[string]bool{
	"github": {
		"opened":           true,
		"reopened":         true,
		"synchronize":      true,
		"ready_for_review": true,
	},
	"gitlab": {
		"open":   true,
		"reopen": true,
		"update": true,
	},
	"gitea": {
		"opened":     true,
		"reopened":   true,
		"synchronized": true,
	},
}

func IsSupportedAction(provider, action string) bool {
	actions, ok := supportedActions[provider]
	if !ok {
		return false
	}
	return actions[action]
}

func IsDraftEvent(provider string, payload map[string]any) bool {
	switch provider {
	case "github":
		if pr, ok := payload["pull_request"].(map[string]any); ok {
			if draft, ok := pr["draft"].(bool); ok && draft {
				return true
			}
		}
	case "gitlab":
		if attrs, ok := payload["object_attributes"].(map[string]any); ok {
			if ws, ok := attrs["work_in_progress"].(bool); ok && ws {
				return true
			}
		}
	case "gitea":
		if pr, ok := payload["pull_request"].(map[string]any); ok {
			if labels, ok := pr["labels"].([]any); ok {
				for _, l := range labels {
					if lm, ok := l.(map[string]any); ok {
						if name, ok := lm["name"].(string); ok && name == "wip" {
							return true
						}
					}
				}
			}
		}
	}
	return false
}

func IsBotSender(provider string, payload map[string]any, botName string) bool {
	var sender string
	switch provider {
	case "github":
		if pr, ok := payload["pull_request"].(map[string]any); ok {
			if user, ok := pr["user"].(map[string]any); ok {
				if login, ok := user["login"].(string); ok {
					sender = login
				}
			}
		}
		if sender == "" {
			if s, ok := payload["sender"].(map[string]any); ok {
				if login, ok := s["login"].(string); ok {
					sender = login
				}
			}
		}
	case "gitlab":
		if user, ok := payload["user"].(map[string]any); ok {
			if username, ok := user["username"].(string); ok {
				sender = username
			}
		}
	case "gitea":
		if pr, ok := payload["pull_request"].(map[string]any); ok {
			if user, ok := pr["user"].(map[string]any); ok {
				if login, ok := user["login"].(string); ok {
					sender = login
				}
			}
		}
	}
	if botName != "" && sender == botName {
		return true
	}
	return false
}

func jsonInt(v any) int {
	switch n := v.(type) {
	case float64:
		return int(n)
	case json.Number:
		i, _ := n.Int64()
		return int(i)
	case int:
		return n
	case string:
		var i int
		_, _ = fmt.Sscanf(n, "%d", &i)
		return i
	}
	return 0
}
