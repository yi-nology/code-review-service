package config

import (
	"strings"
	"testing"
)

func TestParsePort(t *testing.T) {
	tests := []struct {
		in      string
		want    int
		wantErr bool
	}{
		{"8080", 8080, false},
		{":8080", 8080, false},
		{"0.0.0.0:8080", 8080, false},
		{"127.0.0.1:9090", 9090, false},
		{"[::1]:8080", 8080, false},
		{"abc", 0, true},
		{"", 0, true},
	}
	for _, tt := range tests {
		got, err := parsePort(tt.in)
		if (err != nil) != tt.wantErr {
			t.Errorf("parsePort(%q) err=%v wantErr=%v", tt.in, err, tt.wantErr)
		}
		if !tt.wantErr && got != tt.want {
			t.Errorf("parsePort(%q)=%d want %d", tt.in, got, tt.want)
		}
	}
}

func TestValidate_ProdRequiresToken(t *testing.T) {
	cfg := &Config{
		Port:     8080,
		Database: DatabaseConfig{DSN: "postgres://x/y"},
		Auth:     AuthConfig{}, // 空 token
	}
	err := cfg.Validate("prod")
	if err == nil || !strings.Contains(err.Error(), "REVIEW_TOKEN") {
		t.Fatalf("expected REVIEW_TOKEN error, got %v", err)
	}
}

func TestValidate_ProdRejectsInsecure(t *testing.T) {
	cfg := &Config{
		Port:     8080,
		Database: DatabaseConfig{DSN: "postgres://x/y"},
		Auth:     AuthConfig{Insecure: true, Token: "abc123"},
	}
	err := cfg.Validate("prod")
	if err == nil || !strings.Contains(err.Error(), "AUTH_INSECURE") {
		t.Fatalf("expected AUTH_INSECURE error in prod, got %v", err)
	}
}

func TestValidate_DevAllowsInsecureEmptyToken(t *testing.T) {
	cfg := &Config{
		Port:     8080,
		Database: DatabaseConfig{DSN: "postgres://x/y"},
		Auth:     AuthConfig{Insecure: true},
	}
	if err := cfg.Validate("dev"); err != nil {
		t.Fatalf("expected nil err for dev + insecure, got %v", err)
	}
}

func TestValidate_DevEmptyTokenWithoutInsecure(t *testing.T) {
	cfg := &Config{
		Port:     8080,
		Database: DatabaseConfig{DSN: "postgres://x/y"},
		Auth:     AuthConfig{},
	}
	err := cfg.Validate("dev")
	if err == nil || !strings.Contains(err.Error(), "AUTH_INSECURE") {
		t.Fatalf("expected AUTH_INSECURE error in dev, got %v", err)
	}
}

func TestValidate_ProdRejectsDevToken(t *testing.T) {
	cfg := &Config{
		Port:     8080,
		Database: DatabaseConfig{DSN: "postgres://x/y"},
		Auth:     AuthConfig{Token: "dev-token"},
	}
	err := cfg.Validate("prod")
	if err == nil || !strings.Contains(err.Error(), "dev-token") {
		t.Fatalf("expected dev-token error, got %v", err)
	}
}

func TestCurrentEnv_SetGet(t *testing.T) {
	SetCurrentEnv("prod")
	if got := CurrentEnv(); got != "prod" {
		t.Fatalf("expected prod, got %q", got)
	}
	SetCurrentEnv("local")
}
