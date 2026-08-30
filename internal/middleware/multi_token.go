package middleware

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"net/http"
	"strings"
	"time"

	"github.com/yi-nology/code-review-service/internal/domain/auth"
	"github.com/yi-nology/code-review-service/internal/repository"
	"github.com/oekit/concurrency/async"
	"github.com/oekit/logx"

	"github.com/cloudwego/hertz/pkg/app"
)

// 以下为多 Token 鉴权与审计中间件，作为单一 REVIEW_TOKEN 的过渡升级方案。
// POC 推广阶段需要区分「谁在调用」并记录写操作审计，但暂不上完整 RBAC/SSO。

// tokenNameStoreKey 是 Hertz request context 的存储 key（供审计中间件读取）。
const tokenNameStoreKey = "review_service_token_name"

// TokenSource 提供 token 查询能力（解耦具体 repo，便于测试）。
type TokenSource interface {
	GetTokenByHash(ctx context.Context, hash string) (name string, ok bool)
	UpdateLastUsed(ctx context.Context, hash string) error
}

// MultiTokenAuth 是多 Token 鉴权中间件。
// 策略：
//  1. skip 路径（健康检查/swagger/webhook/viewer proxy）直接放行。
//  2. 从 Authorization: Bearer <token> 提取 token，SHA256 哈希后查 DB。
//  3. 命中 → 通过，token name 存入 request context；更新 last_used_at（异步）。
//  4. 未命中 → fallback 到 fallbackToken（兼容现有单 token 配置）。
//  5. 都不匹配 → 401。
func MultiTokenAuth(src TokenSource, fallbackToken string) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		path := string(c.Request.URI().Path())
		if shouldSkipAuth(path) {
			c.Next(ctx)
			return
		}
		token := extractBearerToken(c)
		if token == "" {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}
		hash := hashToken(token)
		if src != nil {
			if name, ok := src.GetTokenByHash(ctx, hash); ok {
				// 命中 DB token：存 name 供审计读取，异步更新 last_used_at。
				c.Set(tokenNameStoreKey, name)
				async.GoSafe(func() {
					bgCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
					defer cancel()
					if err := src.UpdateLastUsed(bgCtx, hash); err != nil {
						logx.CtxWarnf(ctx, "[auth] update token last_used failed: %v", err)
					}
				})
				c.Next(ctx)
				return
			}
		}
		// fallback：兼容单 token 配置（name 标记为 "default"）。
		// constant-time 比较：与 webhook 签名/viewer 代理保持同一安全基线。
		if fallbackToken != "" && subtle.ConstantTimeCompare([]byte(token), []byte(fallbackToken)) == 1 {
			c.Set(tokenNameStoreKey, "default")
			c.Next(ctx)
			return
		}
		c.AbortWithStatus(http.StatusUnauthorized)
	}
}

func extractBearerToken(c *app.RequestContext) string {
	auth := string(c.GetHeader("Authorization"))
	if !strings.HasPrefix(auth, "Bearer ") {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(auth, "Bearer "))
}

func hashToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}

func shouldSkipAuth(path string) bool {
	for _, p := range DefaultSkipPaths {
		if path == p {
			return true
		}
	}
	return strings.HasPrefix(path, "/swagger") ||
		strings.HasPrefix(path, "/openapi.yaml") ||
		strings.HasPrefix(path, "/api/v1/webhooks") ||
		strings.HasPrefix(path, "/api/v1/settings/ocr/viewer/proxy") ||
		// WebSocket upgrade 无法携带 Authorization header，progress WS 有 sameOrigin 校验防 CSRF。
		strings.HasSuffix(path, "/progress")
}

// TokenNameFromRequest 从 Hertz request context 取出鉴权通过的 token 名称。
func TokenNameFromRequest(c *app.RequestContext) string {
	if v, exists := c.Get(tokenNameStoreKey); exists {
		if s, ok := v.(string); ok && s != "" {
			return s
		}
	}
	return "anonymous"
}

// repoTokenSource 适配 repository.AuthRepository → TokenSource 接口。
type repoTokenSource struct {
	repo repository.AuthRepository
}

func (s *repoTokenSource) GetTokenByHash(ctx context.Context, hash string) (string, bool) {
	t, err := s.repo.GetTokenByHash(ctx, hash)
	if err != nil || t == nil {
		return "", false
	}
	// 检查是否过期
	if t.ExpiresAt != nil && t.ExpiresAt.Before(time.Now()) {
		return "", false
	}
	return t.Name, true
}

func (s *repoTokenSource) UpdateLastUsed(ctx context.Context, hash string) error {
	return s.repo.UpdateLastUsed(ctx, hash, time.Now())
}

// NewRepoTokenSource 把 AuthRepository 适配为 TokenSource。
func NewRepoTokenSource(repo repository.AuthRepository) TokenSource {
	return &repoTokenSource{repo: repo}
}

// AuditMiddleware 记录写操作（POST/PUT/DELETE/PATCH）的审计日志。
// GET/HEAD/OPTIONS 不记录（只读操作无审计价值）。
func AuditMiddleware(repo repository.AuthRepository) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		c.Next(ctx)
		method := string(c.Method())
		if method != "POST" && method != "PUT" && method != "DELETE" && method != "PATCH" {
			return
		}
		path := string(c.Request.URI().Path())
		if shouldSkipAuth(path) {
			return
		}
		tokenName := TokenNameFromRequest(c)
		status := c.Response.StatusCode()
		// 异步写审计（不阻塞响应）。
		async.GoSafe(func() {
			bgCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = repo.WriteAudit(bgCtx, &auth.AuditEntry{
				TokenName:  tokenName,
				Action:     method + " " + path,
				Method:     method,
				Path:       path,
				StatusCode: status,
			})
		})
	}
}
