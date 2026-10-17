package redact

import (
	"errors"
	"strings"
	"testing"
)

func TestString_AuthorizationBearer(t *testing.T) {
	in := "Authorization: Bearer ghp_abc123def456ghi789jkl012mno345pqr678"
	out := String(in)
	if strings.Contains(out, "ghp_") {
		t.Fatalf("expected token to be redacted, got %q", out)
	}
	if !strings.Contains(out, "Authorization") {
		t.Fatalf("expected keyword Authorization to remain, got %q", out)
	}
}

// Authorization: Basic 后的值也要打码（logredact 只吃到首个空白，本地规则兜底）。
func TestString_AuthorizationBasic(t *testing.T) {
	in := "Authorization: Basic dXNlcjpwYXNz"
	out := String(in)
	if strings.Contains(out, "dXNlcjpwYXNz") {
		t.Fatalf("expected basic credential to be redacted, got %q", out)
	}
}

func TestString_ApiKey(t *testing.T) {
	in := `{"api_key": "secret-value-12345", "name": "test"}`
	out := String(in)
	if strings.Contains(out, "secret-value-12345") {
		t.Fatalf("expected api_key to be redacted, got %q", out)
	}
}

func TestString_GitHubToken(t *testing.T) {
	in := "remote: Invalid username or password. Token: ghp_abc123def456ghi789jkl012mno345pqr678"
	out := String(in)
	if strings.Contains(out, "ghp_") {
		t.Fatalf("expected ghp_ token to be redacted, got %q", out)
	}
}

func TestString_OpenAIKey(t *testing.T) {
	in := "OpenAI error: invalid api key sk-1234567890abcdefghijklmnop"
	out := String(in)
	if strings.Contains(out, "sk-1234567890") {
		t.Fatalf("expected sk- key to be redacted, got %q", out)
	}
}

func TestString_NoSecretUnchanged(t *testing.T) {
	in := "normal log line without secrets"
	out := String(in)
	if out != in {
		t.Fatalf("expected unchanged, got %q", out)
	}
}

func TestString_EmptyInput(t *testing.T) {
	if out := String(""); out != "" {
		t.Fatalf("expected empty, got %q", out)
	}
}

func TestErrorf_WrapsAndRedacts(t *testing.T) {
	base := errors.New("upstream returned 401: api_key=supersecretkey12345")
	err := Errorf(base, "review failed for %s", "owner/repo")
	msg := err.Error()
	if !strings.Contains(msg, "review failed for owner/repo") {
		t.Fatalf("expected format string applied, got %q", msg)
	}
	if strings.Contains(msg, "supersecretkey12345") {
		t.Fatalf("expected secret in wrapped err to be redacted, got %q", msg)
	}
	// 验证 errors.Unwrap 仍可用
	if !errors.Is(err, base) {
		t.Fatalf("expected errors.Is to find base, got false")
	}
}

func TestString_MultipleSecretsInOneLine(t *testing.T) {
	in := "Authorization: Bearer abc123token Authorization: Basic dXNlcjpwYXNz"
	out := String(in)
	if strings.Contains(out, "abc123token") {
		t.Fatalf("expected Bearer value to be redacted, got %q", out)
	}
	if strings.Contains(out, "dXNlcjpwYXNz") {
		t.Fatalf("expected Basic value to be redacted, got %q", out)
	}
}

// GitLab personal access token（logredact 平台前缀名单未收录，本地补充规则兜底）。
func TestString_GitLabToken(t *testing.T) {
	in := "clone failed for glpat-AbCdEf123456GhIjKlMnOp: remote end hung up"
	out := String(in)
	if strings.Contains(out, "glpat-") {
		t.Fatalf("expected glpat token to be redacted, got %q", out)
	}
}

// logredact 核心新覆盖面：URL 内嵌凭据。
func TestString_URLCredential(t *testing.T) {
	in := "connect postgres://svc:s3cretPw@db.internal:5432/review failed"
	out := String(in)
	if strings.Contains(out, "s3cretPw") {
		t.Fatalf("expected URL password to be redacted, got %q", out)
	}
	if !strings.Contains(out, "postgres://svc:****@") {
		t.Fatalf("expected masked URL form, got %q", out)
	}
}

// logredact 核心新覆盖面：裸 JWT。
func TestString_BareJWT(t *testing.T) {
	in := "auth failed for eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.dBjftJeZ4CVPmB92K27uhbUJU1p1r_wW1gFWFOEjXk"
	out := String(in)
	if strings.Contains(out, "eyJhbGciOiJIUzI1NiJ9") {
		t.Fatalf("expected JWT to be redacted, got %q", out)
	}
}
