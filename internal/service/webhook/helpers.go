package webhook

import (
	"encoding/json"
	"strconv"
)

// gitlab 家族（gitlab/gitcode/tencent_code）同时接受平台原生词（open/reopen/update）
// 与 SDK v0.62+ 规范化词（opened/reopened/updated）——SDK 升级换了 action 词表，
// 真实平台 webhook 的原始 action 也可能两者其一，双收最稳。
var supportedActions = map[string]map[string]bool{
	"github":       {"opened": true, "reopened": true, "synchronize": true, "ready_for_review": true},
	"gitlab":       {"open": true, "reopen": true, "update": true, "opened": true, "reopened": true, "updated": true},
	"gitea":        {"opened": true, "reopened": true, "synchronized": true},
	"gitcode":      {"open": true, "reopen": true, "update": true, "opened": true, "reopened": true, "updated": true},
	"tencent_code": {"open": true, "reopen": true, "update": true, "opened": true, "reopened": true, "updated": true},
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
	case "gitlab", "gitcode", "tencent_code":
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
	case "gerrit":
		// Gerrit change 的 wip 字段标识 work-in-progress 状态。
		if change, ok := payload["change"].(map[string]any); ok {
			if wip, ok := change["wip"].(bool); ok && wip {
				return true
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
	case "gitlab", "gitcode", "tencent_code":
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
	case "gerrit":
		// Gerrit webhooks 插件的 patchset-created 事件中，
		// uploader 在顶层，包含 username 字段。
		if uploader, ok := payload["uploader"].(map[string]any); ok {
			if username, ok := uploader["username"].(string); ok {
				sender = username
			}
		}
		if sender == "" {
			if change, ok := payload["change"].(map[string]any); ok {
				if owner, ok := change["owner"].(map[string]any); ok {
					if username, ok := owner["username"].(string); ok {
						sender = username
					}
				}
			}
		}
	}
	return botName != "" && sender == botName
}

// jsonNumberStr 提取 JSON 数值字段并保留为 string（平台编号全链路统一 string）。
func jsonNumberStr(v any) string {
	switch n := v.(type) {
	case string:
		return n
	case json.Number:
		return n.String()
	case float64:
		return strconv.FormatInt(int64(n), 10)
	case int:
		return strconv.Itoa(n)
	}
	return ""
}
