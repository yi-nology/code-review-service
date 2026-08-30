package middleware

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"net/http"
	"testing"
	"time"

	"github.com/yi-nology/code-review-service/internal/domain/auth"

	"github.com/cloudwego/hertz/pkg/app"
)

// fakeTokenSource 是 TokenSource 的内存实现。
type fakeTokenSource struct {
	names map[string]string // hash -> name
}

func (f *fakeTokenSource) GetTokenByHash(_ context.Context, hash string) (string, bool) {
	n, ok := f.names[hash]
	return n, ok
}

func (f *fakeTokenSource) UpdateLastUsed(_ context.Context, _ string) error { return nil }

// runMultiToken 直接调用中间件，返回 (是否放行, 是否 401 abort, token name)。
func runMultiToken(h app.HandlerFunc, path, bearer string) (passed, aborted bool, tokenName string) {
	ctx := app.NewContext(8)
	ctx.Request.URI().SetPath(path)
	if bearer != "" {
		ctx.Request.Header.Set("Authorization", "Bearer "+bearer)
	}
	h(context.Background(), ctx)
	return !ctx.IsAborted(), ctx.IsAborted(), TokenNameFromRequest(ctx)
}

func TestMultiTokenAuth(t *testing.T) {
	const dbToken = "db-plain-token"
	const fallbackToken = "fallback-secret"
	dbHash := sha256Hex(dbToken)

	src := &fakeTokenSource{names: map[string]string{dbHash: "ci-bot"}}
	mw := MultiTokenAuth(src, fallbackToken)

	t.Run("skip path passes without token", func(t *testing.T) {
		passed, aborted, _ := runMultiToken(mw, "/ping", "")
		if !passed || aborted {
			t.Fatalf("expected /ping to pass without auth")
		}
	})
	t.Run("webhook skip path passes", func(t *testing.T) {
		passed, aborted, _ := runMultiToken(mw, "/api/v1/webhooks/github", "")
		if !passed || aborted {
			t.Fatalf("expected webhook path to pass without auth")
		}
	})
	t.Run("missing bearer rejected", func(t *testing.T) {
		_, aborted, _ := runMultiToken(mw, "/api/v1/reviews", "")
		if !aborted {
			t.Fatalf("expected 401 abort for missing bearer")
		}
	})
	t.Run("non-bearer scheme rejected", func(t *testing.T) {
		ctx := app.NewContext(8)
		ctx.Request.URI().SetPath("/api/v1/reviews")
		ctx.Request.Header.Set("Authorization", "Basic abc")
		mw(context.Background(), ctx)
		if !ctx.IsAborted() {
			t.Fatalf("expected abort for non-Bearer scheme")
		}
	})
	t.Run("db token passes with name", func(t *testing.T) {
		passed, aborted, name := runMultiToken(mw, "/api/v1/reviews", dbToken)
		if !passed || aborted {
			t.Fatalf("expected db token to pass")
		}
		if name != "ci-bot" {
			t.Fatalf("expected token name ci-bot, got %q", name)
		}
	})
	t.Run("fallback token passes as default", func(t *testing.T) {
		passed, aborted, name := runMultiToken(mw, "/api/v1/reviews", fallbackToken)
		if !passed || aborted {
			t.Fatalf("expected fallback token to pass")
		}
		if name != "default" {
			t.Fatalf("expected name default, got %q", name)
		}
	})
	t.Run("unknown token rejected", func(t *testing.T) {
		_, aborted, _ := runMultiToken(mw, "/api/v1/reviews", "wrong-token")
		if !aborted {
			t.Fatalf("expected 401 for unknown token")
		}
	})
	t.Run("nil source still allows fallback", func(t *testing.T) {
		mwNil := MultiTokenAuth(nil, fallbackToken)
		passed, aborted, name := runMultiToken(mwNil, "/api/v1/reviews", fallbackToken)
		if !passed || aborted || name != "default" {
			t.Fatalf("expected nil-source fallback pass, got passed=%v aborted=%v name=%q", passed, aborted, name)
		}
	})
	t.Run("constant-time compare sanity", func(t *testing.T) {
		if subtle.ConstantTimeCompare([]byte(fallbackToken), []byte(fallbackToken)) != 1 {
			t.Fatalf("expected equal tokens to compare 1")
		}
		if subtle.ConstantTimeCompare([]byte(fallbackToken), []byte("fallback-secreT")) != 0 {
			t.Fatalf("expected near-miss tokens to compare 0")
		}
	})
}

func TestShouldSkipAuth(t *testing.T) {
	skip := []string{
		"/ping", "/healthz", "/health", "/ready", "/readyz", "/metrics", "/buildVersion",
		"/swagger/index.html", "/openapi.yaml",
		"/api/v1/webhooks/github", "/api/v1/settings/ocr/viewer/proxy?q=x",
		"/api/v1/reviews/abc/progress",
	}
	for _, p := range skip {
		if !shouldSkipAuth(p) {
			t.Errorf("expected %q to skip auth", p)
		}
	}
	noSkip := []string{
		"/api/v1/reviews", "/api/v1/reviews/abc", "/api/v1/progress/other", "/apiv1/webhooks/x",
	}
	for _, p := range noSkip {
		if shouldSkipAuth(p) {
			t.Errorf("expected %q to NOT skip auth", p)
		}
	}
}

func TestExtractBearerToken(t *testing.T) {
	ctx := app.NewContext(8)
	ctx.Request.Header.Set("Authorization", "Bearer  spaced-token  ")
	if got := extractBearerToken(ctx); got != "spaced-token" {
		t.Fatalf("expected trimmed token, got %q", got)
	}
	ctx2 := app.NewContext(8)
	ctx2.Request.Header.Set("Authorization", "Token abc")
	if got := extractBearerToken(ctx2); got != "" {
		t.Fatalf("expected empty for non-Bearer, got %q", got)
	}
}

func TestHashToken(t *testing.T) {
	got := hashToken("abc")
	want := sha256.Sum256([]byte("abc"))
	if got != hex.EncodeToString(want[:]) {
		t.Fatalf("hashToken mismatch: %s", got)
	}
	if got != hashToken("abc") {
		t.Fatalf("hashToken not deterministic")
	}
}

// fakeAuthRepo 实现 repository.AuthRepository，验证 repoTokenSource 的过期逻辑。
type fakeAuthRepo struct {
	token *auth.Token
}

func (f *fakeAuthRepo) CreateToken(context.Context, *auth.Token) error            { return http.ErrAbortHandler }
func (f *fakeAuthRepo) ListTokens(context.Context) ([]auth.Token, error)          { return nil, nil }
func (f *fakeAuthRepo) DeleteToken(context.Context, uint) error                   { return nil }
func (f *fakeAuthRepo) UpdateLastUsed(context.Context, string, interface{}) error { return nil }
func (f *fakeAuthRepo) WriteAudit(context.Context, *auth.AuditEntry) error { return nil }
func (f *fakeAuthRepo) ListAudit(context.Context, int, int) ([]auth.AuditEntry, int64, error) {
	return nil, 0, nil
}
func (f *fakeAuthRepo) GetTokenByHash(_ context.Context, _ string) (*auth.Token, error) {
	return f.token, nil
}

func TestRepoTokenSourceExpired(t *testing.T) {
	past := time.Now().Add(-time.Hour)
	expired := &auth.Token{Name: "expired", ExpiresAt: &past}
	if _, ok := NewRepoTokenSource(&fakeAuthRepo{token: expired}).GetTokenByHash(context.Background(), "h"); ok {
		t.Fatalf("expected expired token to be rejected")
	}
	future := time.Now().Add(time.Hour)
	live := &auth.Token{Name: "live", ExpiresAt: &future}
	name, ok := NewRepoTokenSource(&fakeAuthRepo{token: live}).GetTokenByHash(context.Background(), "h")
	if !ok || name != "live" {
		t.Fatalf("expected live token accepted, got ok=%v name=%q", ok, name)
	}
	noExpiry := &auth.Token{Name: "forever"}
	if _, ok := NewRepoTokenSource(&fakeAuthRepo{token: noExpiry}).GetTokenByHash(context.Background(), "h"); !ok {
		t.Fatalf("expected token without expiry to be accepted")
	}
}

func sha256Hex(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}
