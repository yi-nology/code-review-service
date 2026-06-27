package config

import (
	"strings"
	"testing"
)

func TestValidateRequiresDatabaseDSN(t *testing.T) {
	cfg := &Config{Port: 8080}

	err := cfg.Validate("local")
	if err == nil || !strings.Contains(err.Error(), "DATABASE_URL") {
		t.Fatalf("expected missing database error, got %v", err)
	}
}

func TestValidateRejectsProdDevToken(t *testing.T) {
	cfg := &Config{
		Port: 8080,
		Auth: AuthConfig{Token: "dev-token"},
		Database: DatabaseConfig{
			DSN: "postgres://review_service:review_service@localhost:5432/review_service?sslmode=disable",
		},
	}

	err := cfg.Validate("prod")
	if err == nil || !strings.Contains(err.Error(), "dev-token") {
		t.Fatalf("expected prod dev-token error, got %v", err)
	}
}

func TestValidateRejectsUnresolvedPlaceholders(t *testing.T) {
	cfg := &Config{
		Port: 8080,
		Auth: AuthConfig{Token: "${REVIEW_TOKEN}"},
		Database: DatabaseConfig{
			DSN: "${DATABASE_URL}",
		},
	}

	err := cfg.Validate("prod")
	if err == nil || !strings.Contains(err.Error(), "unresolved placeholder") {
		t.Fatalf("expected unresolved placeholder error, got %v", err)
	}
}

func TestValidateAcceptsLocalDevToken(t *testing.T) {
	cfg := &Config{
		Port: 8080,
		Auth: AuthConfig{Token: "dev-token"},
		Database: DatabaseConfig{
			DSN:          "postgres://review_service:review_service@localhost:5432/review_service?sslmode=disable",
			MaxIdleConns: 5,
			MaxOpenConns: 10,
		},
	}

	if err := cfg.Validate("local"); err != nil {
		t.Fatalf("expected valid local config, got %v", err)
	}
}
