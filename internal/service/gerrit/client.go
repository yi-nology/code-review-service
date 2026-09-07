// Package gerrit 封装 go-gerrit SDK，提供 Gerrit 平台的评审回写和 Change 查询能力。
//
// 与 git-platform-sdk 的其他平台后端不同，Gerrit 不走 SDK 的 Provider 接口，
// 而是通过 go-gerrit 直接调用 Gerrit REST API。认证使用 HTTP Basic Auth
// （username + HTTP password），这是 Gerrit 的标准认证方式。
package gerrit

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/oekit/logx"
	gogerrit "github.com/andygrunwald/go-gerrit"
)

// Client 封装 go-gerrit 客户端，提供业务层所需的 Gerrit 操作。
type Client struct {
	raw      *gogerrit.Client
	baseURL  string
	username string
}

// NewClient 创建 Gerrit 客户端。
//
// baseURL: Gerrit 实例地址，如 "https://gerrit.example.com"
// token: 认证凭据，格式为 "username:http_password" 或仅 "http_password"（username 默认 "admin"）
// skipTLS: 是否跳过 TLS 证书校验（自签名证书场景）
func NewClient(baseURL, token string, skipTLS bool) (*Client, error) {
	baseURL = strings.TrimRight(baseURL, "/")
	if baseURL == "" {
		return nil, fmt.Errorf("gerrit: base URL is empty")
	}

	username, password := parseToken(token)

	httpClient := &http.Client{}
	if skipTLS {
		httpClient.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, // nolint:gosec
		}
	}

	raw, err := gogerrit.NewClient(context.Background(), baseURL+"/", httpClient)
	if err != nil {
		return nil, fmt.Errorf("gerrit: create client: %w", err)
	}

	raw.Authentication.SetBasicAuth(username, password)

	logx.Info("gerrit client created",
		"base_url", baseURL,
		"username", username,
		"skip_tls", skipTLS,
	)

	return &Client{
		raw:      raw,
		baseURL:  baseURL,
		username: username,
	}, nil
}

// parseToken 解析 token 字段，支持 "username:password" 或仅 "password" 格式。
func parseToken(token string) (username, password string) {
	if idx := strings.Index(token, ":"); idx > 0 {
		return token[:idx], token[idx+1:]
	}
	return "admin", token
}

// BuildCloneURL 构建 Gerrit 认证 clone URL。
// Gerrit 的认证 clone 使用 /a/ 前缀：https://host/a/project.git
func BuildCloneURL(baseURL, project string) string {
	base := strings.TrimRight(baseURL, "/")
	// 如果 base 已经包含 /a/ 路径，不再追加
	if !strings.HasSuffix(base, "/a") && !strings.Contains(base, "/a/") {
		base += "/a"
	}
	// Gerrit project name 可能包含斜杠（如 group/subgroup/project），
	// 但 clone URL 中需要保持原样。
	return fmt.Sprintf("%s/%s.git", base, project)
}

// ParseProjectPath Gerrit 的 project name 不像 GitHub 那样是 owner/repo 两段式。
// 此函数直接返回完整的 project name，owner 为空字符串。
// 这是为了兼容需要 owner+repo 的调用方（如 SDK 接口）。
func ParseProjectPath(project string) (owner, repo string) {
	// Gerrit 没有 owner 概念，project 本身就是完整路径。
	// 为兼容现有 SDK 接口，返回 ("", project)。
	return "", project
}

// FormatProjectPath 将 Gerrit project name 格式化为 review-service 内部使用的
// "owner/repo" 格式。由于 Gerrit 没有 owner，统一使用 "/" + project。
func FormatProjectPath(project string) string {
	return "/" + project
}

// ExtractProjectFromPath 从内部格式 "/project" 还原为 Gerrit project name。
func ExtractProjectFromPath(repoPath string) string {
	return strings.TrimPrefix(repoPath, "/")
}

// DetectGerrit 通过 URL 判断是否为 Gerrit 平台。
// Gerrit 没有 SDK 级别的自动检测，需要基于 URL 特征判断。
func DetectGerrit(remoteURL string) bool {
	// Gerrit URL 通常不包含已知的其他平台域名特征。
	// 这个检测主要用在 DetectAndGetProvider 回退路径中。
	// 实际使用中，Gerrit 平台是通过 DB 中的 provider="gerrit" 显式配置的。
	u, err := url.Parse(remoteURL)
	if err != nil {
		return false
	}
	host := u.Hostname()
	// 不匹配已知的其他平台域名
	knownPlatforms := []string{"github.com", "gitlab.com", "gitea.com", "gitee.com", "gitcode.com"}
	for _, p := range knownPlatforms {
		if host == p || strings.HasSuffix(host, "."+p) {
			return false
		}
	}
	return true
}
