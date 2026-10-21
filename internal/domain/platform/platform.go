// Package platform 是 Git 平台配置的领域类型（#6 迁移）。
package platform

// ProviderGerrit 是 Gerrit 平台的 provider 标识。
//
// Gerrit 不在 go-git-platform SDK 的 backend 列表里：它走独立的 go-gerrit
// 客户端（internal/service/gerrit）。平台注册、连接测试这些「按 provider
// 分派」的路径必须认这条差异——对 gerrit 调 provider.NewProvider 必然拿到
// "platform not registered"，保存会失败、连接测试永远通不过。
const ProviderGerrit = "gerrit"

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
