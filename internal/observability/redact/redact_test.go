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
	if !strings.Contains(out, "Bearer") {
		t.Fatalf("expected keyword Bearer to remain, got %q", out)
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
}
