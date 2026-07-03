// Package platform 是 Git 平台配置的领域类型（#6 迁移）。
package platform

// Platform 表示一个已配置的 Git 平台连接（GitHub/GitLab/Gitea 等）。
type Platform struct {
	Provider       string `json:"provider"`
	InstallationID string `json:"installation_id"`
	BaseURL        string `json:"base_url"`
	Token          string `json:"-"`
	WebhookSecret  string `json:"webhook_secret,omitempty"`
	Enabled        bool   `json:"enabled"`
	SkipTLS        bool   `json:"skip_tls"`
}

// TestResult 是平台连通性测试结果（非持久化 DTO）。
type TestResult struct {
	Connected bool   `json:"connected"`
	Platform  string `json:"platform"`
	UserName  string `json:"user_name"`
	Message   string `json:"message"`
}
